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

type UploaderListResponse struct {
	Data []string `json:"data"`
}

func getBeekeeperNodeList(node_state_api string) (node_list []NodeObj, err error) {
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

func updateUploader(node_list []NodeObj, url string) (updated int, err error) {

	resp, err := http.Get(url + "/user")
	if err != nil {
		err = fmt.Errorf("http.Get error: %s", err.Error())
		return
	}

	api_resp := &UploaderListResponse{}
	err = json.NewDecoder(resp.Body).Decode(api_resp)
	if err != nil {
		err = fmt.Errorf("json.NewDecoder error: %s", err.Error())
		return
	}

	uploader_node_list := api_resp.Data

	existing_users := make(map[string]bool)

	for _, elem := range uploader_node_list {
		existing_users[elem] = true
	}

	for _, node_obj := range node_list {

		node_username := fmt.Sprintf("node-%s", strings.ToLower(node_obj.ID))

		_, ok := existing_users[node_username]
		if ok {
			continue
		}
		fmt.Println("adding user to uploader: ", node_username)
		// add user
		var resp *http.Response
		resp, err = http.Post(url+"/user/"+node_username, "", nil)
		if err != nil {
			err = fmt.Errorf("Adding user to uploader failed: %s\n", err.Error())
			return
		}
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("Adding user to uploader failed.\n")
			return
		}
		updated++
	}
	return
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

// get list of node from beekeeper
// then update RabbitMQ and the uploader if needed.
func Sync() (err error) {
	NODE_STATE_API := os.Getenv("NODE_STATE_API")
	if NODE_STATE_API == "" {
		log.Fatalf("NODE_STATE_API not defined")
	}

	RMQ_URL := os.Getenv("RMQ_URL")
	RMQ_USERNAME := os.Getenv("RMQ_USERNAME")
	RMQ_PASSWORD := os.Getenv("RMQ_PASSWORD")

	UPLOADER_URL := os.Getenv("UPLOADER_URL")

	if RMQ_URL != "" {
		fmt.Printf("RMQ_USERNAME: %s\n", RMQ_USERNAME)
		if RMQ_USERNAME == "" {
			log.Fatalf("RMQ_USERNAME not defined")
		}
	}

	node_list, err := getBeekeeperNodeList(NODE_STATE_API)
	if err != nil {
		err = fmt.Errorf("getBeekeeperNodeList: %s", err.Error())
		return
	}
	for _, node_obj := range node_list {
		fmt.Println("got: ", node_obj.ID)
	}

	updated := 0
	if RMQ_URL != "" {
		updated, err = updateRMQ(node_list, RMQ_URL, RMQ_USERNAME, RMQ_PASSWORD)
		if err != nil {
			err = fmt.Errorf("updateRMQ error: %s", err.Error())
			return
		}
		fmt.Printf("Added %d users to rabbitmq\n", updated)
	} else {
		fmt.Println("RMQ_URL not defined, skipping...")
	}

	if UPLOADER_URL != "" {
		updated, err = updateUploader(node_list, UPLOADER_URL)
		if err != nil {
			err = fmt.Errorf("updateUploader error: %s", err.Error())
			return
		}
		fmt.Printf("Added %d users to uploader\n", updated)

	} else {
		fmt.Println("UPLOADER_URL not defined, skipping...")
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
		fmt.Printf("returned error: %s\n", err.Error())
		http.Error(w, fmt.Sprintf("error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "ok")
}

// for testing:
// kubectl port-forward deployment/beehive-rabbitmq 15672 -n shared

func main() {

	// sync on start once:
	err := Sync()
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
		err = nil
	}

	http.HandleFunc("/sync", syncListener)
	http.HandleFunc("/", rootListener)

	fmt.Println("listening on port 80...")
	http.ListenAndServe(":80", nil)
}
