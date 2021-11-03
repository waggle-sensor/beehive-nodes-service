# beehive-nodes-service
This service gets a list of nodes from beekeeper and updates two beehive serivces. It creates RabbitMQ users for each node if they do not yet exist via the RabbitMQ management API. The uploader also has an API that will be used to add new users.

This service is trigger by a simple GET request on resource `/sync` and returns the repsonse string `ok`.