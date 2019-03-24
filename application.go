package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"

	plex "github.com/jrudio/go-plex-client"
)

var nowPlaying NowPlaying

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

type NowPlaying struct {
	Progress   float64
	ShowTitle  string
	Title      string
	Resolution string
	Season     int64
	Episode    int64
}

type LametricResponse struct {
	Frames []LametricFrame `json:"frames,omitempty"`
}

type LametricFrame struct {
	Text  string `json:"text,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Index int    `json:"index,omitempty"`
}

func (n NowPlaying) ToString() string {
	if n.Resolution == "4K" || n.Resolution == "2160" {
		n.Resolution = "4k"
	} else {
		n.Resolution += "p"
	}

	str := ""

	if n.ShowTitle != "" {
		str += n.ShowTitle
	}

	if n.Title != "" {
		if str == "" {
			str += n.Title
		} else {
			str += fmt.Sprintf(" - %s", n.Title)
		}
	}

	if n.Season != 0 && n.Episode != 0 {
		str += fmt.Sprintf(" - S%02dE%02d", n.Season, n.Episode)
	}

	str += fmt.Sprintf(" [%d%%] (%s)", int(n.Progress*100), n.Resolution)

	if n.Title == "" {
		str = "Nothing is currently playing"
	}

	return str
}

func main() {
	plexConnection, err := plex.New(os.Getenv("PLEX_HOST"), os.Getenv("PLEX_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	// Test your connection to your Plex server
	result, err := plexConnection.Test()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("connection status: %v", result)

	ctrlC := make(chan os.Signal, 1)

	onError := func(err error) {
		log.Println(err)
	}

	events := plex.NewNotificationEvents()
	events.OnPlaying(func(n plex.NotificationContainer) {
		sessionID := n.PlaySessionStateNotification[0].SessionKey

		sessions, err := plexConnection.GetSessions()

		if err != nil {
			log.Printf("failed to fetch sessions on plex server: %v\n", err)
			return
		}

		for _, session := range sessions.MediaContainer.Metadata {
			if sessionID != session.SessionKey {
				continue
			}

			viewOffset, _ := strconv.Atoi(session.ViewOffset)
			duration, _ := strconv.Atoi(session.Duration)

			nowPlaying = NowPlaying{
				ShowTitle:  session.GrandparentTitle,
				Title:      session.Title,
				Progress:   float64(viewOffset) / float64(duration),
				Resolution: session.Media[0].VideoResolution,
				Season:     session.ParentIndex,
				Episode:    session.Index,
			}

			break
		}
	})

	plexConnection.SubscribeToNotifications(events, ctrlC, onError)

	http.HandleFunc("/", handler)
	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	err = http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
	if err != nil {
		log.Fatal(err)
	}

	endWaiter := sync.WaitGroup{}
	endWaiter.Add(1)

	signal.Notify(ctrlC, os.Interrupt)

	go func() {
		<-ctrlC
		endWaiter.Done()
	}()

	endWaiter.Wait()
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	response := LametricResponse{
		Frames: []LametricFrame{
			{
				Text: nowPlaying.ToString(),
				Icon: "i24240",
			},
		},
	}

	body, err := json.Marshal(response)
	if err != nil {
		log.Print(err)
	}

	w.Write(body)
}
