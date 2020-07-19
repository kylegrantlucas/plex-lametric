package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"

	plex "github.com/jrudio/go-plex-client"
	hass "github.com/kylegrantlucas/go-hass"
)

var haClient *hass.Access
var nowPlaying NowPlaying
var plexStatus plex.MetadataV1

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

type NowPlaying struct {
	Progress   float64
	ShowTitle  string
	Title      string
	Resolution *string
	Season     int
	Episode    int
}

type LametricResponse struct {
	Frames []LametricFrame `json:"frames,omitempty"`
}

type LametricFrame struct {
	Text  string `json:"text,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Index int    `json:"index"`
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
		str = "N/A"
	}

	return strings.TrimSpace(str)
}

func main() {
	haClient = hass.NewAccess(os.Getenv("HA_HOST"), os.Getenv("HA_TOKEN"))
	err := haClient.CheckAPI()
	if err != nil {
		panic(err)
	}

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
				Season:     int(session.ParentIndex),
				Episode:    int(session.Index),
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
	appleTVStatus, err := haClient.GetState("media_player.living_room_2")
	if err != nil {
		panic(err)
	}

	plexHAStatus, err := haClient.GetState("media_player.plex_plex_for_apple_tv_living_room")
	if err != nil {
		panic(err)
	}

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

func buildNowPlaying(atv, plexHA hass.State, plexDirect plex.MetadataV1) NowPlaying {
	var nowPlaying NowPlaying

	if atv.State != "playing" && atv.State != "paused" {
		return nowPlaying
	}

	if plexHA.State == "playing" {
		var mediaSeriesTitle string
		var mediaTitle string
		if plexHA.Attributes.MediaSeriesTitle != nil {
			mediaSeriesTitle = *plexHA.Attributes.MediaSeriesTitle
		}

		if plexHA.Attributes.MediaTitle != nil {
			mediaTitle = *plexHA.Attributes.MediaTitle
		}

		if mediaSeriesTitle == plexDirect.GrandparentTitle && mediaTitle == plexDirect.Title {
			viewOffset, _ := strconv.Atoi(plexDirect.ViewOffset)
			duration, _ := strconv.Atoi(plexDirect.Duration)
			resolution := plexDirect.Media[0].VideoResolution

			nowPlaying = NowPlaying{
				ShowTitle:  plexDirect.GrandparentTitle,
				Title:      plexDirect.Title,
				Progress:   float64(viewOffset) / float64(duration),
				Resolution: &resolution,
				Season:     int(plexDirect.ParentIndex),
				Episode:    int(plexDirect.Index),
			}
		} else {
			var episodeNumber int
			var mediaSeason int

			if plexHA.Attributes.MediaSeason != nil {
				mediaSeason = *plexHA.Attributes.MediaSeason
			}
			if plexHA.Attributes.MediaEpisode != nil {
				episodeNumber = *plexHA.Attributes.MediaEpisode
			}

			nowPlaying = NowPlaying{
				ShowTitle: mediaSeriesTitle,
				Title:     mediaTitle,
				Progress:  float64(*plexHA.Attributes.MediaPosition) / float64(*plexHA.Attributes.MediaDuration),
				Season:    mediaSeason,
				Episode:   episodeNumber,
			}
		}
	} else {
		if atv.Attributes.MediaAlbumName != nil && *atv.Attributes.MediaAlbumName == "FuboTV" {
			nowPlaying = NowPlaying{
				Title: *atv.Attributes.MediaAlbumName,
			}
		} else {
			mediaArtist := ""
			mediaTitle := ""

			if atv.Attributes.MediaArtist != nil {
				mediaArtist = *atv.Attributes.MediaArtist
			}

			if atv.Attributes.MediaTitle != nil {
				mediaTitle = *atv.Attributes.MediaTitle
			}

			mediaPosition := 0.0
			mediaDuration := 0.0

			if atv.Attributes.MediaPosition != nil {
				mediaPosition = float64(*atv.Attributes.MediaPosition)
			}

			if atv.Attributes.MediaDuration != nil {
				mediaDuration = float64(*atv.Attributes.MediaDuration)
			}

			nowPlaying = NowPlaying{
				Title:    mediaArtist + " " + mediaTitle,
				Progress: float64(mediaPosition) / float64(mediaDuration),
			}
		}
	}

	return nowPlaying
}
