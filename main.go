package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
)

type Node struct {
	ID string `json:"id"`
}

type Metrics struct {
	RabbitmqUsersUpdated     int
	UploadServerUsersUpdated int
}

func getBeekeeperNodes(nodeStateURL string) ([]Node, error) {
	resp, err := http.Get(nodeStateURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get node state data: %s", err.Error())
	}

	var nodeState struct {
		Data []Node
	}
	if err := json.NewDecoder(resp.Body).Decode(&nodeState); err != nil {
		return nil, fmt.Errorf("failed to decode node state data: %s", err.Error())
	}
	return nodeState.Data, nil
}

func updateRabbitmqUser(rmqclient *rabbithole.Client, username string) error {
	if _, err := rmqclient.PutUser(username, rabbithole.UserSettings{Password: "secret"}); err != nil {
		return fmt.Errorf("failed to setup user %s: %s", username, err.Error())
	}

	if _, err := rmqclient.UpdatePermissionsIn("/", username, rabbithole.Permissions{
		Configure: "^amq.gen",
		Read:      ".*",
		Write:     ".*",
	}); err != nil {
		return fmt.Errorf("failed to update permissions for user %s: %s", username, err.Error())
	}

	return nil
}

func updateUploader(nodes []Node, url string, metrics *Metrics) error {
	resp, err := http.Get(url + "/user")
	if err != nil {
		return fmt.Errorf("failed to get user data: %s", err.Error())
	}

	var apiResp struct {
		Data []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode user data: %s", err.Error())
	}

	uploadServerUsernames := apiResp.Data

	hasUsername := make(map[string]bool)

	for _, username := range uploadServerUsernames {
		hasUsername[username] = true
	}

	for _, node := range nodes {
		username := fmt.Sprintf("node-%s", strings.ToLower(node.ID))

		if hasUsername[username] {
			continue
		}

		fmt.Println("adding user to uploader: ", username)

		if resp, err := http.Post(url+"/user/"+username, "", nil); err != nil {
			return fmt.Errorf("adding user to uploader failed: %s", err.Error())
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("adding user to uploader failed")
		}

		metrics.UploadServerUsersUpdated++
	}

	return nil
}

func updateRabbitmq(nodes []Node, url string, username string, password string, metrics *Metrics) error {
	rmqClient, err := rabbithole.NewClient(url, username, password)
	if err != nil {
		return fmt.Errorf("failed to create rabbithole client: %s", err.Error())
	}

	existingUsers, err := rmqClient.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to list rabbitmq users: %s", err.Error())
	}

	hasUsername := map[string]bool{}
	for _, user := range existingUsers {
		hasUsername[user.Name] = true
	}

	// add missing users to RMQ
	for _, node := range nodes {
		username := fmt.Sprintf("node-%s", strings.ToLower(node.ID))

		if hasUsername[username] {
			continue
		}

		fmt.Printf("adding rmq user %s\n", username)

		if err := updateRabbitmqUser(rmqClient, username); err != nil {
			return fmt.Errorf("failed to update rabbitmq user: %s", err.Error())
		}

		metrics.RabbitmqUsersUpdated++
	}

	return nil
}

// Updates the RabbitMQ and uploader server node users.
func syncUsers(config *Config) error {
	metrics := &Metrics{}

	nodes, err := getBeekeeperNodes(config.NodeStateURL)
	if err != nil {
		return fmt.Errorf("getBeekeeperNodeList: %s", err.Error())
	}
	for _, node := range nodes {
		fmt.Printf("got: %s\n", node.ID)
	}

	if err := updateRabbitmq(nodes, config.RabbitmqURL, config.RabbitmqUsername, config.RabbitmqPassword, metrics); err == nil {
		fmt.Printf("added %d users to rabbitmq\n", metrics.RabbitmqUsersUpdated)
	} else {
		fmt.Printf("failed to sync rabbitmq users: %s\n", err.Error())
	}

	if err := updateUploader(nodes, config.UploadServerURL, metrics); err == nil {
		fmt.Printf("added %d users to upload server\n", metrics.UploadServerUsersUpdated)
	} else {
		fmt.Printf("failed to sync upload server users: %s\n", err.Error())
	}

	return nil
}

// for testing:
// kubectl port-forward deployment/beehive-rabbitmq 15672 -n shared

func main() {
	config := mustGetConfigFromEnv()

	// Sync once at startup and then every 5 minutes.
	if err := syncUsers(config); err != nil {
		fmt.Printf("failed to sync users: %s\n", err.Error())
	}

	ticker := time.NewTicker(5 * time.Minute)

	for range ticker.C {
		if err := syncUsers(config); err != nil {
			fmt.Printf("failed to sync users: %s\n", err.Error())
		}
	}
}
