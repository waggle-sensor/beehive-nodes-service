package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
)

type Account struct {
	Username string
	Active   bool
}

func getAccounts(nodeStateURL string) ([]Account, error) {
	usernameActive := map[string]bool{}

	var nodeState struct {
		Data []struct {
			ID string `json:"id"`
		}
	}

	if err := getJSON(nodeStateURL, &nodeState); err != nil {
		return nil, fmt.Errorf("failed to get node state data from beekeeper: %s", err.Error())
	}

	for _, item := range nodeState.Data {
		username := "node-" + strings.ToLower(item.ID)
		usernameActive[username] = true
	}

	var adminDatabaseAccounts []struct {
		Username string `json:"user"`
		Active   bool   `json:"active"`
	}

	if err := getJSON("https://auth.sagecontinuum.org/service-node-users", &adminDatabaseAccounts); err != nil {
		return nil, fmt.Errorf("failed to get accounts data from beehive: %s", err.Error())
	}

	for _, item := range adminDatabaseAccounts {
		usernameActive[item.Username] = item.Active
	}

	accounts := []Account{}

	for username, active := range usernameActive {
		accounts = append(accounts, Account{
			Username: username,
			Active:   active,
		})
	}

	return accounts, nil
}

func getJSON(url string, data any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(data)
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

func updateUploaderAccounts(accounts []Account, url string) error {
	fmt.Printf("updating upload server accounts\n")

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

	accountsAdded := 0

	for _, account := range accounts {
		if account.Active && !hasUsername[account.Username] {
			fmt.Println("adding user to uploader: ", account.Username)

			// TODO(sean) Review how this is implemented on the upload server side! Strange to have an
			// unauthenticated post like this...
			if resp, err := http.Post(url+"/user/"+account.Username, "", nil); err != nil {
				return fmt.Errorf("adding user to upload server failed: %s", err.Error())
			} else if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("adding user to upload server failed")
			}

			accountsAdded++
		}
	}

	fmt.Printf("%d accounts added to upload server\n\n", accountsAdded)

	return nil
}

func updateRabbitmqAccounts(accounts []Account, url string, username string, password string) error {
	fmt.Printf("updating rabbitmq server accounts\n")

	rmqClient, err := rabbithole.NewClient(url, username, password)
	if err != nil {
		return fmt.Errorf("failed to create rabbithole client: %s", err.Error())
	}

	rabbitmqUsers, err := rmqClient.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to list rabbitmq users: %s", err.Error())
	}

	hasUsername := map[string]bool{}
	for _, user := range rabbitmqUsers {
		hasUsername[user.Name] = true
	}

	accountsAdded := 0

	for _, account := range accounts {
		if account.Active && !hasUsername[account.Username] {
			fmt.Printf("adding rabbitmq user %s\n", account.Username)

			if err := updateRabbitmqUser(rmqClient, account.Username); err != nil {
				return fmt.Errorf("failed to update rabbitmq user: %s", err.Error())
			}

			accountsAdded++
		}
	}

	fmt.Printf("\n%d accounts added to rabbitmq\n\n", accountsAdded)

	return nil
}

// Updates the RabbitMQ and uploader server node users.
func syncUsers(config *Config) error {
	accounts, err := getAccounts(config.NodeStateURL)
	if err != nil {
		return fmt.Errorf("failed to get accounts: %s", err.Error())
	}

	fmt.Printf("found the following accounts:\nusername\tactive\n")

	for _, account := range accounts {
		fmt.Printf("%s\t%v\n", account.Username, account.Active)
	}
	fmt.Printf("\n")

	if err := updateRabbitmqAccounts(accounts, config.RabbitmqURL, config.RabbitmqUsername, config.RabbitmqPassword); err != nil {
		fmt.Printf("failed to sync rabbitmq users: %s\n", err.Error())
	}

	if err := updateUploaderAccounts(accounts, config.UploadServerURL); err != nil {
		fmt.Printf("failed to sync upload server users: %s\n", err.Error())
	}

	return nil
}

func main() {
	config := mustGetConfigFromEnv()

	// Sync once immediately at startup.
	if err := syncUsers(config); err != nil {
		fmt.Printf("failed to sync users: %s\n", err.Error())
	}

	// Sync every 5 minutes.
	ticker := time.NewTicker(5 * time.Minute)

	for range ticker.C {
		if err := syncUsers(config); err != nil {
			fmt.Printf("failed to sync users: %s\n", err.Error())
		}
	}
}
