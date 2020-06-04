package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	plex "github.com/jrudio/go-plex-client"
)

var nowPlaying NowPlaying
var plexStatus plex.MetadawtaV1

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

type NowPlaying struct {
	Progress   float64
	ShowTitle  string
	Title      string
	Resolution *string
	Season     int64
	Episode    int64
}

type LametricResponse struct {
	Frames []LametricFrame `json:"frames,omitempty"`
}

type LametricFrame struct {
	Text  string `json:"text,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Index int    `json:"index"`
}

type PlexHomeAssistantState struct {
	Attributes struct {
		AppName                string    `json:"app_name"`
		EntityPicture          string    `json:"entity_picture"`
		FriendlyName           string    `json:"friendly_name"`
		IsVolumeMuted          bool      `json:"is_volume_muted"`
		MediaContentID         int       `json:"media_content_id"`
		MediaContentRating     string    `json:"media_content_rating"`
		MediaContentType       string    `json:"media_content_type"`
		MediaDuration          int       `json:"media_duration"`
		MediaEpisode           string    `json:"media_episode"`
		MediaLibraryName       string    `json:"media_library_name"`
		MediaPosition          int       `json:"media_position"`
		MediaPositionUpdatedAt time.Time `json:"media_position_updated_at"`
		MediaSeason            int       `json:"media_season"`
		MediaSeriesTitle       string    `json:"media_series_title"`
		MediaTitle             string    `json:"media_title"`
		SessionUsername        string    `json:"session_username"`
		Summary                string    `json:"summary"`
		SupportedFeatures      int       `json:"supported_features"`
		VolumeLevel            int       `json:"volume_level"`
	} `json:"attributes"`
	Context struct {
		ID       string      `json:"id"`
		ParentID interface{} `json:"parent_id"`
		UserID   interface{} `json:"user_id"`
	} `json:"context"`
	EntityID    string    `json:"entity_id"`
	LastChanged time.Time `json:"last_changed"`
	LastUpdated time.Time `json:"last_updated"`
	State       string    `json:"state"`
}

type AppleTVHomeAssistantState struct {
	Attributes struct {
		AppID                  string    `json:"app_id"`
		EntityPicture          string    `json:"entity_picture"`
		FriendlyName           string    `json:"friendly_name"`
		Icon                   string    `json:"icon"`
		MediaAlbumName         string    `json:"media_album_name"`
		MediaArtist            string    `json:"media_artist"`
		MediaContentType       string    `json:"media_content_type"`
		MediaDuration          int       `json:"media_duration"`
		MediaPosition          int       `json:"media_position"`
		MediaPositionUpdatedAt time.Time `json:"media_position_updated_at"`
		MediaTitle             string    `json:"media_title"`
		SupportedFeatures      int       `json:"supported_features"`
	} `json:"attributes"`
	Context struct {
		ID       string      `json:"id"`
		ParentID interface{} `json:"parent_id"`
		UserID   interface{} `json:"user_id"`
	} `json:"context"`
	EntityID    string    `json:"entity_id"`
	LastChanged time.Time `json:"last_changed"`
	LastUpdated time.Time `json:"last_updated"`
	State       string    `json:"state"`
}

func (n NowPlaying) ToString() string {
	if n.Title == "FuboTV" {
		return "FuboTV · Live"
	}

	if n.Resolution != nil {
		res := *n.Resolution
		if strings.EqualFold(res, "4K") || res == "2160" || strings.EqualFold(res, "2160p") {
			*n.Resolution = "4k"
		} else {
			if !strings.EqualFold(res[len(res)-1:], "p") {
				*n.Resolution += "p"
			}
		}
	}

	str := ""

	if n.ShowTitle != "" {
		str += n.ShowTitle
	}

	if n.Season != 0 && n.Episode != 0 {
		str += fmt.Sprintf(" S%02d · E%02d:", n.Season, n.Episode)
	}

	if n.Title != "" {
		if str == "" {
			str += n.Title
		} else {
			str += fmt.Sprintf(" %s", n.Title)
		}
	}

	if n.Resolution != nil {
		str += fmt.Sprintf(" [%d%%] (%s)", int(n.Progress*100), *n.Resolution)
	} else {
		str += fmt.Sprintf(" [%d%%]", int(n.Progress*100))
	}

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

			plexStatus = session

			nowPlaying = NowPlaying{
				ShowTitle:  session.GrandparentTitle,
				Title:      session.Title,
				Progress:   float64(viewOffset) / float64(duration),
				Resolution: &session.Media[0].VideoResolution,
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
	appleTVStatus := getAppleTVStatus()
	plexHAStatus := getPlexHomeAssistantStatus()

	nowPlaying = buildNowPlaying(appleTVStatus, plexHAStatus, plexStatus)

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

func getPlexHomeAssistantStatus() PlexHomeAssistantState {
	var s PlexHomeAssistantState
	url := os.Getenv("HA_HOST") + "/api/states/media_player.plex_plex_for_apple_tv_living_room"

	// Create a Bearer string by appending string access token
	var bearer = "Bearer " + os.Getenv("HA_TOKEN")

	// Create a new request using http
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
	}

	// add authorization header to the req
	req.Header.Add("Authorization", bearer)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
	}

	err = json.Unmarshal(body, &s)
	if err != nil {
		log.Print(err)
	}

	return s
}

func getAppleTVStatus() AppleTVHomeAssistantState {
	var s AppleTVHomeAssistantState
	url := os.Getenv("HA_HOST") + "/api/states/media_player.living_room_2"

	// Create a Bearer string by appending string access token
	var bearer = "Bearer " + os.Getenv("HA_TOKEN")

	// Create a new request using http
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
	}

	// add authorization header to the req
	req.Header.Add("Authorization", bearer)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error on response.\n[ERRO] -", err)
	}

	err = json.Unmarshal(body, &s)
	if err != nil {
		log.Print(err)
	}

	err = json.Unmarshal(body, &s)
	if err != nil {
		log.Print(err)
	}

	return s
}

func buildNowPlaying(atv AppleTVHomeAssistantState, plexHA PlexHomeAssistantState, plexDirect plex.MetadataV1) NowPlaying {
	var nowPlaying NowPlaying

	if atv.State != "playing" && atv.State != "paused" {
		return nowPlaying
	}

	if plexHA.State == "playing" {
		if plexHA.Attributes.MediaSeriesTitle == plexDirect.GrandparentTitle && plexHA.Attributes.MediaTitle == plexDirect.Title {
			viewOffset, _ := strconv.Atoi(plexDirect.ViewOffset)
			duration, _ := strconv.Atoi(plexDirect.Duration)
			resolution := plexDirect.Media[0].VideoResolution

			nowPlaying = NowPlaying{
				ShowTitle:  plexDirect.GrandparentTitle,
				Title:      plexDirect.Title,
				Progress:   float64(viewOffset) / float64(duration),
				Resolution: &resolution,
				Season:     plexDirect.ParentIndex,
				Episode:    plexDirect.Index,
			}
		} else {
			episode, _ := strconv.Atoi(plexHA.Attributes.MediaEpisode)
			nowPlaying = NowPlaying{
				ShowTitle: plexHA.Attributes.MediaSeriesTitle,
				Title:     plexHA.Attributes.MediaTitle,
				Progress:  float64(plexHA.Attributes.MediaPosition) / float64(plexHA.Attributes.MediaDuration),
				Season:    int64(plexHA.Attributes.MediaSeason),
				Episode:   int64(episode),
			}
		}
	} else {
		if atv.Attributes.MediaAlbumName == "FuboTV" {
			nowPlaying = NowPlaying{
				Title: atv.Attributes.MediaAlbumName,
			}
		} else {
			nowPlaying = NowPlaying{
				Title:    atv.Attributes.MediaArtist + " " + atv.Attributes.MediaTitle,
				Progress: float64(atv.Attributes.MediaPosition) / float64(atv.Attributes.MediaDuration),
			}
		}
	}

	return nowPlaying
}
