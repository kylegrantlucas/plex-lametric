package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"

	plex "github.com/jrudio/go-plex-client"
	"github.com/kylegrantlucas/discord-lametric/models"
	"github.com/kylegrantlucas/discord-lametric/services"
)

var nowPlaying NowPlaying
var config models.Config
var lametricClient lametric.Client

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// var err error
	// var file string
	// if os.Getenv("CONFIG_FILE") != "" {
	// 	file = os.Getenv("CONFIG_FILE")
	// } else {
	// 	file = "./config.default.json"
	// }

	// configJSON, err := ioutil.ReadFile(file)
	// if err != nil {
	// 	log.Fatalf("Error opening Discord session: %v", err)
	// }

	// err = json.Unmarshal(configJSON, &config)
	// if err != nil {
	// 	log.Fatalf("Error opening Discord session: %v", err)
	// }

	lametricClient = lametric.Client{Host: config.LaMetricIP, APIKey: config.LaMetricAPIKey}
}

type NowPlaying struct {
	Progress   float64
	ShowTitle  string
	Title      string
	Resolution string
	Season     int64
	Episode    int64
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
	plexConnection, err := plex.New("http://192.168.1.120:32400", "scdfmVAfjXHdXZqaQwyx")
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
		// mediaID := n.PlaySessionStateNotification[0].RatingKey
		sessionID := n.PlaySessionStateNotification[0].SessionKey
		// var title, userID, username string

		sessions, err := plexConnection.GetSessions()

		if err != nil {
			log.Printf("failed to fetch sessions on plex server: %v\n", err)
			return
		}

		for _, session := range sessions.MediaContainer.Metadata {
			if sessionID != session.SessionKey {
				continue
			}

			// userID = session.User.ID
			// username = session.User.Title

			// spew.Dump(session)
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

			log.Print(nowPlaying.ToString())

			break
		}

		// metadata, err := plexConnection.GetMetadata(mediaID)

		// if err != nil {
		// 	log.Printf("failed to get metadata for key %s: %v\n", mediaID, err)
		// } else {
		// 	title = metadata.MediaContainer.Metadata[0].Title
		// }

		// log.Printf("user %s (id: %s) has started playing %s (id: %s)\n", username, userID, title, mediaID)
	})

	plexConnection.SubscribeToNotifications(events, ctrlC, onError)

	endWaiter := sync.WaitGroup{}
	endWaiter.Add(1)

	signal.Notify(ctrlC, os.Interrupt)

	go func() {
		<-ctrlC
		endWaiter.Done()
	}()

	endWaiter.Wait()
	// discord, err := discordgo.New(config.DiscordEmail, config.DiscordPassword, config.DiscordToken)
	// if err != nil {
	// 	log.Fatalf("Error opening Discord session: %v", err)
	// }

	// allowedServersMap, err = buildAllowedServerList(discord, config)
	// if err != nil {
	// 	log.Fatalf("Error building server list: %v", err)
	// }

	// allowedChannelsMap, err = buildAllowedChannelList(discord, allowedServersMap, config)
	// if err != nil {
	// 	log.Fatalf("Error building channel list: %v", err)
	// }

	// log.Printf("allowing on servers: %v", allowedServersMap)
	// log.Printf("allowing on channels: %v", allowedChannelsMap)

	// // Register messageCreate callback
	// discord.AddHandler(messageCreate)

	// // Open the websocket and begin listening.
	// err = discord.Open()
	// if err != nil {
	// 	log.Fatalf("Error opening Discord session: %v", err)
	// }

	// sc := make(chan os.Signal, 1)
	// signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	// <-sc

	// // Cleanly close down the Discord session.
	// discord.Close()
}

// func buildLaMetricNotification(message string, channelConfig *models.ChannelConfig) lametric.Notification {
// 	notif := lametric.Notification{
// 		IconType: channelConfig.IconType,
// 		Priority: channelConfig.Priority,
// 		Model: lametric.Model{
// 			Frames: []lametric.Frame{
// 				{
// 					Icon: channelConfig.Icon,
// 					Text: message,
// 				},
// 			},
// 		},
// 	}

// 	if channelConfig.Sound != nil {
// 		notif.Model.Sound = &lametric.Sound{
// 			Category: channelConfig.Sound.Category,
// 			ID:       channelConfig.Sound.ID,
// 			Repeat:   channelConfig.Sound.Repeat,
// 		}
// 	}

// 	return notif
// }

// func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
// 	if m.Author.ID == s.State.User.ID {
// 		return
// 	}

// 	if serverIsAllowed(m.GuildID) && channelIsAllowed(m.ChannelID) {
// 		channelConfig := fetchChannelConfig(m.ChannelID)
// 		if channelConfig != nil {
// 			err := lametricClient.Notify(
// 				buildLaMetricNotification(m.Message.Content, channelConfig),
// 			)
// 			if err != nil {
// 				log.Print(err)
// 			}
// 		} else {
// 			log.Printf("channel is not configured: %v, %v", allowedChannelsMap[m.ChannelID], m.ChannelID)
// 		}
// 	}
// }

// func fetchChannelConfig(channelID string) *models.ChannelConfig {
// 	for _, server := range config.Servers {
// 		for _, channel := range server.Channels {
// 			if channel.Name == allowedChannelsMap[channelID] {
// 				return &channel
// 			}
// 		}
// 	}

// 	return nil
// }

// func channelIsAllowed(id string) bool {
// 	if allowedChannelsMap[id] != "" {
// 		return true
// 	}

// 	return false
// }

// func serverIsAllowed(id string) bool {
// 	if allowedServersMap[id] != "" {
// 		return true
// 	}

// 	return false
// }

// func buildAllowedChannelList(discord *discordgo.Session, serversMap map[string]string, config models.Config) (map[string]string, error) {
// 	ids := map[string]string{}
// 	for serverID := range serversMap {
// 		channels, err := discord.GuildChannels(serverID)
// 		if err != nil {
// 			log.Fatalf("Error building channel list: %v", err)
// 			return map[string]string{}, err
// 		}

// 		for _, ch := range channels {
// 			for _, server := range config.Servers {
// 				for _, allowed := range server.Channels {
// 					if ch.Name == allowed.Name {
// 						ids[ch.ID] = ch.Name
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return ids, nil
// }

// func buildAllowedServerList(discord *discordgo.Session, config models.Config) (map[string]string, error) {
// 	ids := map[string]string{}
// 	guilds, err := discord.UserGuilds(50, "", "")
// 	if err != nil {
// 		return map[string]string{}, err
// 	}

// 	for _, guild := range guilds {
// 		for _, allowed := range config.Servers {
// 			if guild.Name == allowed.Name {
// 				ids[guild.ID] = guild.Name
// 			}
// 		}
// 	}

// 	return ids, nil
// }
