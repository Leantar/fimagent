package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/Leantar/fimagent/models"
	"github.com/Leantar/fimagent/modules/watcher"
	"github.com/Leantar/fimproto/proto"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io/fs"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Host        string `yaml:"host"`
	Port        int64  `yaml:"port"`
	CertFile    string `yaml:"cert_file"`
	CertKeyFile string `yaml:"cert_key_file"`
	CaFile      string `yaml:"ca_file"`
}

type Agent struct {
	conn   *grpc.ClientConn
	client proto.FimClient
	conf   Config
}

func New(config Config) *Agent {
	return &Agent{
		conf: config,
	}
}

func (a *Agent) Connect() error {
	creds, err := createGrpcCredentials(a.conf.CertFile, a.conf.CertKeyFile, a.conf.CaFile)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(a.conf.Host, strconv.FormatInt(a.conf.Port, 10))

	a.conn, err = grpc.Dial(address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return err
	}
	log.Info().Msgf("connected to %s", address)

	a.client = proto.NewFimClient(a.conn)

	return nil
}

func (a *Agent) Run() error {
	ctx := context.Background()
	info, err := a.client.GetStartupInfo(ctx, &proto.Empty{})
	if err != nil {
		return err
	}

	if info.CreateBaseline {
		err = a.createBaseline(info.WatchedPaths)
	} else if info.UpdateBaseline {
		err = a.updateBaseline(info.WatchedPaths)
	} else {
		err = a.reportFsStatus(info.WatchedPaths)
	}
	if err != nil {
		return err
	}

	return a.watchFsEvents(info.WatchedPaths)
}

func (a *Agent) Stop() error {
	log.Info().Msg("stopping agent")
	return a.conn.Close()
}

func (a *Agent) watchFsEvents(watchedPaths []string) error {
	w := watcher.NewDebounced()

	for _, path := range watchedPaths {
		err := w.AddRecursiveWatch(path)
		if err != nil {
			return err
		}
	}

	ctx := context.Background()
	for event := range w.Events {
		var err error
		var obj models.FsObject

		if event.Kind() == watcher.KindDelete {
			obj = models.FsObject{
				Path: event.Path,
			}
		} else {
			obj, err = models.NewFsObject(event.Path)
			if err != nil {
				log.Warn().Caller().Err(err).Msg("failed to create new models")
				continue
			}
		}

		evt := &proto.Event{
			Kind:     event.Kind(),
			IssuedAt: time.Now().Unix(),
			FsObject: &proto.FsObject{
				Path:     obj.Path,
				Hash:     obj.Hash,
				Created:  obj.Created,
				Modified: obj.Modified,
				Uid:      obj.Uid,
				Gid:      obj.Gid,
				Mode:     obj.Mode,
			},
		}

		_, err = a.client.ReportFsEvent(ctx, evt)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) collectFsObjects(watchedPaths []string) ([]models.FsObject, error) {
	var objs []models.FsObject

	for _, path := range watchedPaths {
		path = filepath.Clean(path)

		stat, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			// File/Folder does not exist. Server will generate "DELETE" alert
			continue
		}
		if err != nil {
			return nil, err
		}

		if stat.IsDir() {
			err = filepath.WalkDir(path, walk(&objs))
			if err != nil {
				return nil, err
			}
		} else {
			obj, err := models.NewFsObject(path)
			if err != nil {
				return nil, err
			}

			objs = append(objs, obj)
		}
	}

	return objs, nil
}

func (a *Agent) createBaseline(watchedPaths []string) (err error) {
	objs, err := a.collectFsObjects(watchedPaths)
	if err != nil {
		return
	}

	stream, err := a.client.CreateBaseline(context.Background())
	if err != nil {
		return
	}

	for _, obj := range objs {
		file := proto.FsObject{
			Path:     obj.Path,
			Hash:     obj.Hash,
			Created:  obj.Created,
			Modified: obj.Modified,
			Uid:      obj.Uid,
			Gid:      obj.Gid,
			Mode:     obj.Mode,
		}
		if err = stream.Send(&file); err != nil {
			return
		}
	}
	_, err = stream.CloseAndRecv()
	return
}

func (a *Agent) updateBaseline(watchedPaths []string) (err error) {
	objs, err := a.collectFsObjects(watchedPaths)
	if err != nil {
		return
	}

	stream, err := a.client.UpdateBaseline(context.Background())
	if err != nil {
		return
	}

	for _, obj := range objs {
		file := proto.FsObject{
			Path:     obj.Path,
			Hash:     obj.Hash,
			Created:  obj.Created,
			Modified: obj.Modified,
			Uid:      obj.Uid,
			Gid:      obj.Gid,
			Mode:     obj.Mode,
		}
		if err = stream.Send(&file); err != nil {
			return
		}
	}
	_, err = stream.CloseAndRecv()
	return
}

func (a *Agent) reportFsStatus(watchedPaths []string) (err error) {
	objs, err := a.collectFsObjects(watchedPaths)
	if err != nil {
		return
	}

	stream, err := a.client.ReportFsStatus(context.Background())
	if err != nil {
		return
	}

	for _, obj := range objs {
		file := proto.FsObject{
			Path:     obj.Path,
			Hash:     obj.Hash,
			Created:  obj.Created,
			Modified: obj.Modified,
			Uid:      obj.Uid,
			Gid:      obj.Gid,
			Mode:     obj.Mode,
		}
		if err = stream.Send(&file); err != nil {
			return
		}
	}
	_, err = stream.CloseAndRecv()
	return
}

func walk(objs *[]models.FsObject) fs.WalkDirFunc {
	return func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		obj, err := models.NewFsObject(path)
		if err != nil {
			return err
		}

		*objs = append(*objs, obj)
		return nil
	}
}

func createGrpcCredentials(certPath, keyPath, caPath string) (credentials.TransportCredentials, error) {
	caFile, err := filepath.Abs(caPath)
	if err != nil {
		return nil, err
	}

	caBytes, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	certFile, err := filepath.Abs(certPath)
	if err != nil {
		return nil, err
	}

	keyFile, err := filepath.Abs(keyPath)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(caBytes)
	if !ok {
		return nil, fmt.Errorf("failed to parse %s", caFile)
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
	}), nil
}
