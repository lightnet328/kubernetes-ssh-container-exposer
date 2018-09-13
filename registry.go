package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kelseyhightower/envconfig"
	crud "github.com/tg123/sshpiper/sshpiperd/upstream/mysql/crud"
	"go.uber.org/zap"
)

type Registry struct {
	database *sql.DB
}

type Config struct {
	Port     int    `default:"3306"`
	Host     string `default:"localhost"`
	User     string `default:"root"`
	Password string `default:""`
	Database string `default:"sshpiper"`
}

type Upstream struct {
	Name                string
	Username            string
	Address             string
	SSHPiperPrivateKey  string
	DownstreamPublicKey string
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) ConnectDatabase() error {
	var conf Config
	var err error
	err = envconfig.Process("KSCE_MYSQL", &conf)
	if err != nil {
		return err
	}
	logger.Info("MySQL Config", zap.String("user", conf.User), zap.String("host", conf.Host), zap.Int("port", conf.Port), zap.String("database", conf.Database))
	source := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", conf.User, conf.Password, conf.Host, conf.Port, conf.Database)
	r.database, err = sql.Open("mysql", source)
	return err
}

func (r *Registry) IsConnected() bool {
	return r.database != nil
}

func (r *Registry) truncate(table string, hasForeignKey bool, ignoreForeignKeyChecks bool) error {
	var tx *sql.Tx
	var err error
	if tx, err = r.database.Begin(); err != nil {
		return err
	}
	if hasForeignKey && ignoreForeignKeyChecks {
		if _, err = tx.Exec("set foreign_key_checks = 0;"); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(fmt.Sprintf("truncate table %s;", table)); err != nil {
		return err
	}
	if hasForeignKey && ignoreForeignKeyChecks {
		if _, err = tx.Exec("set foreign_key_checks = 1;"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Registry) TruncateAll() error {
	var err error
	type table struct {
		name          string
		hasForeignKey bool
	}
	tables := []table{
		{"pubkey_prikey_map", false},
		{"pubkey_upstream_map", false},
		{"user_upstream_map", false},
		{"private_keys", true},
		{"public_keys", true},
		{"server", true},
		{"upstream", true},
	}
	for _, table := range tables {
		if err = r.truncate(table.name, table.hasForeignKey, table.hasForeignKey); err != nil {
			return err
		}
	}
	logger.Info("Database truncated")
	return nil
}

func (r *Registry) RegisterUpstream(upstream *Upstream) (*Upstream, error) {
	var err error
	var serverID int64
	var upstreamID int64
	var privateKeyID int64
	var publicKeyID int64

	s := crud.NewServer(r.database)
	if rec, err := s.GetFirstByAddress(upstream.Address); err == nil {
		if rec != nil {
			serverID = rec.Id
		} else {
			serverID, err = s.Post(&crud.ServerRecord{Name: upstream.Name, Address: upstream.Address})
			err = s.Commit()
		}
	} else {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	u := crud.NewUpstream(r.database)
	if rec, err := u.GetFirstByServerId(serverID); err == nil {
		if rec != nil {
			upstreamID = rec.Id
		} else {
			upstreamID, err = u.Post(&crud.UpstreamRecord{Name: upstream.Name, ServerId: serverID, Username: "root"})
			err = u.Commit()
		}
	} else {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	uum := crud.NewUserUpstreamMap(r.database)
	if rec, err := uum.GetFirstByUpstreamId(upstreamID); err == nil && rec == nil {
		_, err = uum.Post(&crud.UserUpstreamMapRecord{UpstreamId: upstreamID, Username: upstream.Username})
		err = uum.Commit()
	} else {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	prv := crud.NewPrivateKeys(r.database)
	if rec, err := prv.GetFirstByData(upstream.SSHPiperPrivateKey); err == nil && rec == nil {
		if rec != nil {
			privateKeyID = rec.Id
		} else {
			privateKeyID, err = prv.Post(&crud.PrivateKeysRecord{Name: upstream.Name, Data: upstream.SSHPiperPrivateKey})
			err = prv.Commit()
		}
	} else {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	pub := crud.NewPublicKeys(r.database)
	if rec, err := pub.GetFirstByData(upstream.DownstreamPublicKey); err == nil && rec == nil {
		if rec != nil {
			publicKeyID = rec.Id
		} else {
			publicKeyID, err = pub.Post(&crud.PublicKeysRecord{Name: upstream.Name, Data: upstream.DownstreamPublicKey})
			err = pub.Commit()
		}
	} else {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	ppm := crud.NewPubkeyPrikeyMap(r.database)
	if rec, err := ppm.GetFirstByPrivateKeyId(privateKeyID); err == nil && rec == nil {
		_, err = ppm.Post(&crud.PubkeyPrikeyMapRecord{PrivateKeyId: privateKeyID, PubkeyId: publicKeyID})
		err = ppm.Commit()
	} else {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	logger.Info("Upstream registered", zap.String("name", upstream.Name), zap.String("username", upstream.Username), zap.String("public_key", upstream.DownstreamPublicKey))

	return nil, err
}
