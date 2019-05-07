# Kubernetes SSH Container Exposer

Kubernetes SSH Container Exposer registers the SSH container information in the database and helps to proxy by SSHPiper.

```
                                               Kubernetes
                              +------------------------------------------+
                              |                        Namespace=bob     |
                              | +----------------+ +-------------------+ |
                              | |                | |                   | |
                              | | +------------+ | | +---------------+ | |
                              | | |            | | | |               | | |
                              | | |   MySQL    | | | | SSH Container | | |
+---------+                   | | |            | | | |               | | |
|         |                   | | +------------+ | | +-------^-------+ | |
|   Bob   +--+ssh -l bob+---+ | |                | |         |         | |
|         |                 | | | +------------+ | +---------|---------+ |
+---------+                 | | | |            | |           |           |
                            +-----> SSH Piper  +-------------+           |
+---------+                 | | | |            | |           |           |
|         |                 | | | +------------+ | +---------|---------+ |
|  Alice  +--+ssh -l alice+-+ | |                | |         |         | |
|         |                   | | +------------+ | | +-------v-------+ | |
+---------+                   | | |            | | | |               | | |
                              | | |    KSCE    | | | | SSH Container | | |
                              | | |            | | | |               | | |
                              | | +------------+ | | +---------------+ | |
                              | |                | |                   | |
                              | +----------------+ +-------------------+ |
                              |                       Namespace=alice    |
                              +------------------------------------------+
```

## Installing the Chart

To install the chart with the release name `ksce`:

```bash
$ git clone git@github.com:lightnet328/kubernetes-ssh-container-exposer.git
$ cd kubernetes-ssh-container-exposer
$ helm dep build
$ helm inspect values . > ksce.yaml
# Edit the values files
$ vim ksce.yaml
$ helm install --name ksce --values ksce.yaml .
```

## Uninstalling the Chart

To uninstall/delete the `ksce` deployment:

```bash
$ helm delete ksce --purge
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the KSCE chart and their default values.

| Parameter                   | Description                   | Default                                        |
| --------------------------- | ----------------------------- | ---------------------------------------------- |
| `image.repository`          | KSCE Image name               | `lightnet328/kubernetes-ssh-container-exposer` |
| `image.tag`                 | KSCE Image tag                | `0.3.0`                                        |
| `image.pullPolicy`          | Image pull policy             | `IfNotPresent`                                 |
| `sshpiper.image.repository` | SSHPiper Image name           | `farmer1992/sshpiperd`                         |
| `sshpiper.image.tag`        | SSHPiper Image tag            | `latest`                                       |
| `sshpiper.image.pullPolicy` | Image pull policy             | `IfNotPresent`                                 |
| `sshpiper.service.type`     | Kubernetes Service type       | `NodePort`                                     |
| `sshpiper.service.port`     | Kubernetes Service port       | `2222`                                         |
| `mysql.mysqlRootPassword`   | Password for the `root` user. | `9M0ujgwXes879BqQ`                             |

## Configuration on ssh container

```bash
# Create public and private keys to communicate between ssh container and sshpiper
$ ssh-keygen -f id_rsa
$ SSHPIPER_PRIVATE_KEY=`cat id_rsa.pub | base64`
$ SSHPIPER_PUBLIC_KEY=`cat id_rsa | base64`
$ PUBLIC_KEY=`cat $HOME/.ssh/id_rsa.pub | base64`
$ echo "
apiVersion: v1
kind: Pod
metadata:
  name: ssh-pod
  labels:
    app: ssh-pod
spec:
  containers:
    - name: ssh-pod
      image: ssh-pod:latest
      ports:
        - containerPort: 22
      volumeMounts:
        - mountPath: /root/.ssh/
          name: authorized-keys
  volumes:
  - name: authorized-keys
    secret:
      secretName: ssh-pod-sshpiper-publickey
---
apiVersion: v1
kind: Secret
metadata:
  name: ssh-pod-sshpiper-publickey
type: Opaque
data:
  authorized_keys: $SSHPIPER_PUBLIC_KEY
---
apiVersion: v1
kind: Secret
metadata:
  name: ssh-pod
type: Opaque
data:
  sshpiper_id_rsa: $SSHPIPER_PRIVATE_KEY
  downstream_id_rsa.pub: $PUBLIC_KEY
" > ssh-pod.yml
$ kubectl create -f ssh-pod.yml
```