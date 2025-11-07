package control_plane

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	redisservice "rabbit-cp-proxy/redis_service"
	"sync"
)

var (
	// key be like username:password->rabbit_url
	backendAddrTable = make(map[string]string)
	tableMutex       = &sync.RWMutex{}
)
var auth_token = os.Getenv("AUTH_TOKEN")
var controlPlaneURL = os.Getenv("CONTROL_PLANE_URL")

func GetBackendAddress(username, password string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/infra/rabbit/route-table?username=%s&password=%s&auth_token=%s", controlPlaneURL, username, password, auth_token)
	key := fmt.Sprintf("%s:%s", username, password)
	tableMutex.RLock()
	addr, ok := backendAddrTable[key]
	tableMutex.RUnlock()
	if ok {
		return addr, nil
	}
	ctx := context.Background()
	redisService := redisservice.GetInstance()

	client := redisService.GetClient(ctx)
	backendAddr, err := client.Get(ctx, key).Result()
	if err != nil {
		log.Println("control plane error:", err)
		return "", err
	}
	if backendAddr != "" {
		tableMutex.Lock()
		backendAddrTable[key] = backendAddr
		tableMutex.Unlock()
		return backendAddr, nil
	}

	response, err := http.Get(url)
	if err != nil {
		log.Println("control plane error:", err)
		return "", err
	}
	defer response.Body.Close()
	var result struct {
		Message string `json:"message"`
		Success bool   `json:"success"`
		Backend string `json:"backend_url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	tableMutex.Lock()
	backendAddrTable[key] = result.Backend
	tableMutex.Unlock()
	return result.Backend, nil
}

func StartSubscriber() {
	var req struct {
		OldKey  string `json:"old_key"`
		NewKey  string `json:"new_key"`
		Backend string `json:"backend_url"`
	}
	ctx := context.Background()
	redisService := redisservice.GetInstance()
	client := redisService.GetClient(ctx)

	Subscriber := client.Subscribe(ctx, "update-table")
	go func() {
		for {
			msg, err := Subscriber.ReceiveMessage(ctx)
			if err != nil {
				fmt.Println("Error receiving message:", err)
				return
			}
			err = json.Unmarshal([]byte(msg.Payload), &req)
			if err != nil {
				fmt.Println("Error unmarshalling message:", err)
				return
			}
			if req.OldKey != "" {
				if _, exists := backendAddrTable[req.OldKey]; exists {
					delete(backendAddrTable, req.OldKey)
					fmt.Println("Deleted old key:", req.OldKey)
				}
			}

			if req.NewKey != "" {
				tableMutex.Lock()
				backendAddrTable[req.NewKey] = req.Backend
				tableMutex.Unlock()
				fmt.Println("Updated mapping:", req.NewKey, "->", req.Backend)
			}
		}

	}()
}
