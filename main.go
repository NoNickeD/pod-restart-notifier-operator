package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	discordWebhookURLEnvVar = "DISCORD_WEBHOOK_URL"
	teamsWebhookURLEnvVar   = "TEAMS_WEBHOOK_URL"
	slackWebhookURLEnvVar   = "SLACK_WEBHOOK_URL"
)

var (
	discordWebhookURL string
	teamsWebhookURL   string
	slackWebhookURL   string
)

var podState = make(map[string]struct {
	RestartCount int32
	LastAlert    time.Time
})

func main() {
	discordWebhookURL = os.Getenv(discordWebhookURLEnvVar)
	teamsWebhookURL = os.Getenv(teamsWebhookURLEnvVar)
	slackWebhookURL = os.Getenv(slackWebhookURLEnvVar)

	http.HandleFunc("/healthz", healthz)
	http.HandleFunc("/readyz", readyz)
	go func() {
		log.Println("Starting HTTP server...")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	log.Println("Starting application...")

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error obtaining in-cluster config: %s", err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %s", err.Error())
	}

	for {
		log.Println("Fetching pod information...")
		pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatalf("Error fetching pods: %s", err.Error())
		}

		for _, pod := range pods.Items {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				key := pod.Namespace + "/" + pod.Name + "/" + containerStatus.Name

				state, exists := podState[key]
				if !exists {
					log.Printf("New pod detected: %s/%s/%s", pod.Namespace, pod.Name, containerStatus.Name)
					podState[key] = struct {
						RestartCount int32
						LastAlert    time.Time
					}{
						RestartCount: containerStatus.RestartCount,
						LastAlert:    time.Now(),
					}
					continue
				}

				restartDifference := containerStatus.RestartCount - state.RestartCount

				log.Printf("Checking pod: %s, container: %s, restartDifference: %d, containerRestarts: %d, stateRestarts: %d\n",
					pod.Name, containerStatus.Name, restartDifference, containerStatus.RestartCount, state.RestartCount)

				if restartDifference > 0 {
					sendNotification(pod.Name, restartDifference)
					podState[key] = struct {
						RestartCount int32
						LastAlert    time.Time
					}{
						RestartCount: containerStatus.RestartCount,
						LastAlert:    time.Now(),
					}
				} else {
					state.RestartCount = containerStatus.RestartCount
					podState[key] = state
				}
			}
		}
		log.Println("Sleeping for 1 minute before next check...")
		time.Sleep(1 * time.Minute)
	}
}

func sendNotification(podName string, restartDifference int32) {
	message := fmt.Sprintf("Pod %s has restarted %d times!", podName, restartDifference)

	// Send to Discord
	if discordWebhookURL != "" {
		discordPayload := fmt.Sprintf(`{"content": "%s"}`, message)
		postMessage(discordWebhookURL, discordPayload)
	}

	// Send to Microsoft Teams
	if teamsWebhookURL != "" {
		teamsPayload := fmt.Sprintf(`{
			"@type": "MessageCard",
			"@context": "http://schema.org/extensions",
			"summary": "Pod Restart Notification",
			"themeColor": "0078D7",
			"text": "%s"
		}`, message)
		postMessage(teamsWebhookURL, teamsPayload)
	}

	// Send to Slack
	if slackWebhookURL != "" {
		slackPayload := fmt.Sprintf(`{
			"text": "%s"
		}`, message)
		postMessage(slackWebhookURL, slackPayload)
	}
}

func postMessage(url, payload string) {
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		log.Printf("Error sending message: %s", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		log.Printf("Failed to send message with status code: %d", resp.StatusCode)
	}
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Healthy"))
}

func readyz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}
