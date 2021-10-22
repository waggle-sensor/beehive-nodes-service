package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	rabbithole "github.com/michaelklishin/rabbit-hole"
)

type NodeObj struct {
	ID string `json:"id"`
}

type APIResponse struct {
	Data []NodeObj
}

func getNodeList() []NodeObj {
	api_resp := &APIResponse{}

	resp, err := http.Get(os.Getenv("NODE_STATE_API"))
	if err != nil {
		log.Fatalln(err)
	}

	err = json.NewDecoder(resp.Body).Decode(api_resp)
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}

	return api_resp.Data

}

func updateRabbitmqUser(rmqclient *rabbithole.Client, username string) error {
	if _, err := rmqclient.PutUser(username, rabbithole.UserSettings{Password: "secret"}); err != nil {
		return err
	}

	if _, err := rmqclient.UpdatePermissionsIn("/", username, rabbithole.Permissions{
		Configure: "^amq.gen",
		Read:      ".*",
		Write:     ".*",
	}); err != nil {
		return err
	}

	return nil
}

// for testing:
// kubectl port-forward deployment/beehive-rabbitmq 15671 -n shared

func main() {

	fmt.Println("hello world")

	list := getNodeList()
	for _, value := range list {
		fmt.Println("got: ", value.ID)
	}

	cer, err := tls.LoadX509KeyPair("/etc/tls/cert.pem", "/etc/tls/key.pem")
	if err != nil {
		log.Println(err)
		return
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cer}}

	//var tlsConfig *tls.Config

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	//rmqc, err := rabbithole.NewTLSClient("https://127.0.0.1:15672", "guest", "guest", transport)
	rmqc, err := rabbithole.NewTLSClient("https://127.0.0.1:15672", "beehive-master", "beehive-master", transport)
	if err != nil {
		log.Fatalln(err)
	}

	xs, err := rmqc.ListUsers()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("ListUsers:")
	fmt.Println(xs)
	fmt.Println("done")

	err = updateRabbitmqUser(rmqc, "SURYALAPTOP00000")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("added")
}
