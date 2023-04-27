package main

import (
	"context"
	"dblocal/model"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/go-sql-driver/mysql"

	"github.com/astaxie/beego/orm"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type DatabaseConnection struct {
	Alias    string    `yaml:"alias,omitempty"`
	Host     string    `yaml:"host,omitempty"`
	Port     string    `yaml:"port,omitempty"`
	User     string    `yaml:"user,omitempty"`
	Password string    `yaml:"password,omitempty"`
	Schema   string    `yaml:"schema,omitempty"`
	Tunnel   SSHTunnel `yaml:"tunnel,omitempty"`
}

type SSHTunnel struct {
	Host     string `yaml:"host,omitempty"`
	Port     string `yaml:"port,omitempty"`
	User     string `yaml:"user,omitempty"`
	Password string `yaml:"password,omitempty"`
	Key      string `yaml:"key,omitempty"`
}
type ViaSSHDialer struct {
	client *ssh.Client
}

func (dialer *ViaSSHDialer) Dial(addr string) (net.Conn, error) {
	return dialer.client.Dial("tcp", addr)
}
func init() {
	var agentClient agent.Agent
	var conn net.Conn
	var signer ssh.Signer
	var sshConfig *ssh.ClientConfig
	var err error
	c := DatabaseConnection{
		Alias:    "default",
		Host:     "", // db host
		Port:     "", // db port
		User:     "", // db username
		Password: "", // db password
		Schema:   "",
		Tunnel: SSHTunnel{
			Host: "", // ssh host
			Port: "", // ssh port
			User: "", // ssh username
		},
	}
	if len(c.Tunnel.Key) == 0 {
		home, _ := os.UserHomeDir()
		c.Tunnel.Key = home + "/.ssh/id_rsa"
	}
	pemBytes, err := ioutil.ReadFile(c.Tunnel.Key)
	if err != nil {
		fmt.Println("error occured in reading ssh file", err)
		return
	}

	signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte("password")) // if password is there
	// signer, err = ssh.ParsePrivateKey(pemBytes) // without password on ssh file
	if err != nil {
		fmt.Println("error occured in parsing the ssh file", err)
		return
	}

	sshConfig = &ssh.ClientConfig{
		User:            c.Tunnel.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err = net.Dial("tcp", c.Tunnel.Host+":"+c.Tunnel.Port)
	if err != nil {
		fmt.Println("error occured in connecting to ssh url", err)
		return
	}
	defer conn.Close()
	agentClient = agent.NewClient(conn)
	if agentClient == nil {
		return
	}

	if len(c.Tunnel.Password) != 0 {
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(c.Tunnel.Password))
	}

	sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeysCallback(agentClient.Signers))

	sshcon, err := ssh.Dial("tcp", c.Tunnel.Host+":"+c.Tunnel.Port, sshConfig)
	if err != nil {
		fmt.Println("error occured in ssh connecting the ssh url", err)
		return
	}
	mysql.RegisterDialContext("mysql+tcp", func(ctx context.Context, addr string) (net.Conn, error) {
		return (&ViaSSHDialer{sshcon}).Dial(addr)
	})
	connectionString := fmt.Sprintf("%s:%s@mysql+tcp(%s)/%s", c.User, c.Password, c.Host+":"+c.Port, c.Schema)
	if err = orm.RegisterDataBase(c.Alias, "mysql", connectionString); err != nil {
		fmt.Println("error occured in registering db", err)
		return
	}
	// register model
	orm.RegisterModel(new(model.DbTable)) //register model
}

func main() {
	o := orm.NewOrm()
	var maps []orm.Params
	_, err := o.QueryTable(new(model.DbTable)).Values(&maps)
	if err != nil {
		fmt.Println("error occured in running the query", err)
		return
	}
	fmt.Println(maps)
	defer o.Rollback()
}
