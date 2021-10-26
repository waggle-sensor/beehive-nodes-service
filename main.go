package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	rabbithole "github.com/michaelklishin/rabbit-hole"
)

type NodeObj struct {
	ID string `json:"id"`
}

type APIResponse struct {
	Data []NodeObj
}

func getNodeList(node_state_api string) (node_list []NodeObj, err error) {
	api_resp := &APIResponse{}

	resp, err := http.Get(node_state_api)
	if err != nil {
		err = fmt.Errorf("http.Get error: %s", err.Error())
		return
	}

	err = json.NewDecoder(resp.Body).Decode(api_resp)
	if err != nil {
		err = fmt.Errorf("json.NewDecoder error: %s", err.Error())
		return
	}

	node_list = api_resp.Data
	return

}

func updateRabbitmqUser(rmqclient *rabbithole.Client, username string) (err error) {
	if _, err = rmqclient.PutUser(username, rabbithole.UserSettings{Password: "secret"}); err != nil {
		err = fmt.Errorf("rmqclient.PutUser error: %s", err.Error())
		return
	}

	if _, err = rmqclient.UpdatePermissionsIn("/", username, rabbithole.Permissions{
		Configure: "^amq.gen",
		Read:      ".*",
		Write:     ".*",
	}); err != nil {
		err = fmt.Errorf("rmqclient.UpdatePermissionsIn error: %s", err.Error())
		return
	}

	return nil
}

func updateRMQ(node_list []NodeObj, url string, username string, password string) (updated int, err error) {
	rmqc, err := rabbithole.NewClient(url, username, password)
	if err != nil {
		err = fmt.Errorf("rabbithole.NewClient error: %s", err.Error())
		return
	}

	var xs []rabbithole.UserInfo
	xs, err = rmqc.ListUsers()
	if err != nil {
		err = fmt.Errorf("ListUsers error: %s", err.Error())
		return
	}
	fmt.Println("Beekeeper reported nodes:")

	existing_users := make(map[string]rabbithole.UserInfo, 0)
	for _, elem := range xs {
		fmt.Println(elem.Name)
		existing_users[strings.ToLower(elem.Name)] = elem
	}
	fmt.Println("----")
	// add missing users to RMQ
	for _, node_obj := range node_list {

		node_rmq_user := fmt.Sprintf("node-%s", strings.ToLower(node_obj.ID))

		_, ok := existing_users[node_rmq_user]
		if ok {
			continue
		}

		fmt.Printf("adding rmq user %s...\n", node_rmq_user)
		err = updateRabbitmqUser(rmqc, node_rmq_user)
		if err != nil {
			err = fmt.Errorf("updateRabbitmqUser error: %s", err.Error())
			return
		}
		updated++
	}

	fmt.Printf("%d rmq users added.\n", updated)

	return
}

func Sync() (err error) {
	NODE_STATE_API := os.Getenv("NODE_STATE_API")

	RMQ_URL := os.Getenv("RMQ_URL")
	RMQ_USERNAME := os.Getenv("RMQ_USERNAME")
	RMQ_PASSWORD := os.Getenv("RMQ_PASSWORD")

	fmt.Printf("RMQ_USERNAME: %s\n", RMQ_USERNAME)
	if RMQ_USERNAME == "" {
		log.Fatalf("RMQ_USERNAME not defined")
	}

	node_list, err := getNodeList(NODE_STATE_API)
	if err != nil {
		err = fmt.Errorf("getNodeList: %s", err.Error())
	}
	for _, node_obj := range node_list {
		fmt.Println("got: ", node_obj.ID)
	}

	_, err = updateRMQ(node_list, RMQ_URL, RMQ_USERNAME, RMQ_PASSWORD)
	if err != nil {
		err = fmt.Errorf("updateRMQ: %s", err.Error())
	}

	return
}

func rootListener(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "alive")
}

func syncListener(w http.ResponseWriter, req *http.Request) {
	fmt.Println("/sync was called")
	err := Sync()

	if err != nil {
		http.Error(w, fmt.Sprintf("error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "ok")
}

// for testing:
// kubectl port-forward deployment/beehive-rabbitmq 15672 -n shared

func main() {

	// opn start once:
	_ = Sync()

	http.HandleFunc("/sync", syncListener)
	http.HandleFunc("/", rootListener)

	fmt.Println("listening on port 80...")
	http.ListenAndServe(":80", nil)
}
