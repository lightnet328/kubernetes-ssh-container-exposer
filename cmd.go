package main

import (
	"encoding/base64"
	"log"

	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var SSHServicePort int32 = 22

type Services []v1.Service
type GroupedServices map[string]Services
type Keys struct {
	SSHPiperPrivateKey  string
	DownstreamPublicKey string
}
type ServiceKeys map[string]map[string]Keys

func newClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func getServiceList(client kubernetes.Interface) (*v1.ServiceList, error) {
	return client.CoreV1().Services("").List(meta_v1.ListOptions{})
}

func hasPort(servicePorts []v1.ServicePort, port int32) bool {
	for _, servicePort := range servicePorts {
		if servicePort.Port == port {
			return true
		}
	}
	return false
}

func filterSSHServices(services Services) Services {
	var SSHServices Services
	for _, service := range services {
		if hasPort(service.Spec.Ports, SSHServicePort) {
			SSHServices = append(SSHServices, service)
		}
	}
	return SSHServices
}

func groupByNamespace(services Services) GroupedServices {
	groupedServices := GroupedServices{}
	for _, service := range services {
		groupedServices[service.Namespace] = append(groupedServices[service.Namespace], service)
	}
	return groupedServices
}

func getKeys(client kubernetes.Interface, namespace string, name string) (Keys, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return Keys{}, err
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(secret.Data["downstream_id_rsa.pub"])
	if err != nil {
		return Keys{}, err
	}
	return Keys{
		SSHPiperPrivateKey:  string(secret.Data["sshpiper_id_rsa"]),
		DownstreamPublicKey: base64.StdEncoding.EncodeToString(publicKey.Marshal()),
	}, nil
}

func getServiceKeys(client kubernetes.Interface, services GroupedServices) (ServiceKeys, error) {
	serviceKeys := ServiceKeys{}
	for namespace, services := range services {
		serviceKeys[namespace] = map[string]Keys{}
		for _, service := range services {
			keys, err := getKeys(client, namespace, service.Name)
			if err != nil {
				return ServiceKeys{}, err
			}
			serviceKeys[namespace][service.Name] = keys
		}
	}
	return serviceKeys, nil
}

func registerServices(registry *Registry, services GroupedServices, serviceKeys ServiceKeys) error {
	for namespace, services := range services {
		switch len(services) {
		case 1:
			service := services[0]
			keys := serviceKeys[namespace][service.Name]
			if _, err := registry.RegisterUpstream(&Upstream{
				Name:                service.Name,
				Username:            service.Namespace,
				Address:             service.Spec.ClusterIP,
				SSHPiperPrivateKey:  keys.SSHPiperPrivateKey,
				DownstreamPublicKey: keys.DownstreamPublicKey,
			}); err != nil {
				return err
			}
		case 2:
			log.Printf("Registration process is skipped. There are two or more services using %d port for service in namespace %s.\n", SSHServicePort, namespace)
		default:
			continue
		}
	}
	return nil
}

func initialize() error {
	var services GroupedServices
	registry := NewRegistry()
	if err := registry.ConnectDatabase(); err != nil {
		return err
	}
	if err := registry.TruncateAll(); err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}
	serviceList, err := getServiceList(client)
	if err != nil {
		return err
	}

	services = groupByNamespace(filterSSHServices(serviceList.Items))
	serviceKeys, err := getServiceKeys(client, services)
	if err != nil {
		return err
	}
	return registerServices(registry, services, serviceKeys)
}

func main() {
	err := initialize()
	if err != nil {
		log.Fatal(err)
	}
}
