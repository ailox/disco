package discord

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"layeh.com/gopus"
)

const (
	CHANNELS   = 2
	FRAME_RATE = 48000
	FRAME_SIZE = 960
	MAX_BYTES  = FRAME_SIZE * CHANNELS * 2
)

var buffer = make([][]byte, 0)

var cmdid string

func Airhorn(token string) {
	if token == "" {
		fmt.Println("No token provided. please use -t <bot token>")
		return
	}

	// Load the sound file.
	err := loadSound()
	if err != nil {
		fmt.Println("Error loading sound: ", err)
		fmt.Println("Please copy airhorn.dca to this directory.")
		return
	}

	// Create a new Discord session using the provided bot token.
	discordConnection, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	// Register ready as a callback for the ready events.
	discordConnection.AddHandler(ready)

	// Register guildCreate as a callback for the guildCreate events.
	discordConnection.AddHandler(guildCreate)

	discordConnection.AddHandler(commandCreate)

	// We need information about guilds (which includes their channels),
	// messages and voice states.
	discordConnection.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

	// Open the websocket and begin listening.
	err = discordConnection.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Airhorn is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	discordConnection.Close()
}

// This function will be called (due to AddHandler above) when the bot receives
// the "ready" event from Discord.
func ready(s *discordgo.Session, event *discordgo.Ready) {

	// Set the playing status.
	s.UpdateGameStatus(0, "/airhorn [url]")

	// register the /airhorn command
	_, err := s.ApplicationCommandCreate(s.State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        "airhorn",
		Description: "Plays an airhorn sound in your current voice channel",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "The URL of the audio to play",
				Required:    true,
			},
		},
	})
	if err != nil {
		fmt.Println("Error creating application command: ", err)
	}
}

// This function will be called (due to AddHandler above) every time a new
// guild is joined.
func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {

	if event.Guild.Unavailable {
		return
	}

	//for _, channel := range event.Guild.Channels {
	//	if channel.ID == event.Guild.ID {
	//		_, _ = s.ChannelMessageSend(channel.ID, "Airhorn is ready! Type !airhorn while in a voice channel to play a sound.")
	//		return
	//	}
	//}
}

func commandCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {

	if i.ApplicationCommandData().Name == "airhorn" {

		options := i.ApplicationCommandData().Options
		if len(options) != 1 {
			message := "Please provide a valid URL."
			interactionResponse := &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message,
				},
			}
			s.InteractionRespond(i.Interaction, interactionResponse)
			return
		}

		urlOption := options[0]

		if urlOption.Name != "url" {
			message := "Please provide a valid URL."
			interactionResponse := &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message,
				},
			}
			s.InteractionRespond(i.Interaction, interactionResponse)
			return
		}

		// Find the channel that the message came from.
		c, err := s.State.Channel(i.ChannelID)
		if err != nil {
			// Could not find channel.
			return
		}

		// Find the guild for that channel.
		g, err := s.State.Guild(c.GuildID)
		if err != nil {
			// Could not find guild.
			return
		}

		// Look for the message sender in that guild's current voice states.
		for _, vs := range g.VoiceStates {
			if vs.UserID == i.Member.User.ID {

				// "Thinking ..."
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				})

				//err = playSound(s, g.ID, vs.ChannelID)
				err = playYoutube(s, g.ID, vs.ChannelID, urlOption.StringValue())
				if err != nil {
					fmt.Println("Error playing sound:", err)
					// reply to the interaction
					_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Error playing sound",
						},
					})
					return
				}

				// reply to the interaction
				_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "🎺",
					},
				})
				return
			}
		}

		// reply to the interaction
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You need to be in a voice channel to use this command!",
			},
		})
	}
}

// loadSound attempts to load an encoded sound file from disk.
func loadSound() error {

	file, err := os.Open("airhorn.dca")
	if err != nil {
		fmt.Println("Error opening dca file :", err)
		return err
	}

	var opuslen int16

	for {
		// Read opus frame length from dca file.
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return.
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err := file.Close()
			if err != nil {
				return err
			}
			return nil
		}

		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Read encoded pcm from dca file.
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Append encoded pcm data to the buffer.
		buffer = append(buffer, InBuf)
	}
}

// playSound plays the current buffer to the provided channel.
func playSound(s *discordgo.Session, guildID, channelID string) (err error) {

	// Join the provided voice channel.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return err
	}

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(250 * time.Millisecond)

	// Start speaking.
	vc.Speaking(true)

	// Send the buffer data.
	for _, buff := range buffer {
		vc.OpusSend <- buff
	}

	// Stop speaking
	vc.Speaking(false)

	// Sleep for a specificed amount of time before ending.
	time.Sleep(250 * time.Millisecond)

	// Disconnect from the provided voice channel.
	vc.Disconnect()

	return nil
}

func playYoutube(s *discordgo.Session, guildID, channelID, youtubeURL string) (err error) {

	// Join the provided voice channel.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return err
	}

	streamURL, err := getStreamURL(youtubeURL)
	if err != nil {
		return err
	}

	fmt.Println("Playing stream: ", streamURL)

	err = playStream(vc, streamURL)
	if err != nil {
		return err
	}

	// Sleep for a specificed amount of time before ending.
	time.Sleep(250 * time.Millisecond)

	// Disconnect from the provided voice channel.
	vc.Disconnect()

	return nil
}

func playStream(vc *discordgo.VoiceConnection, streamURL string) error {
	ffmpeg := exec.Command("ffmpeg", "-i", streamURL, "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")
	ffmpegout, err := ffmpeg.StdoutPipe()
	if err != nil {
		// wrap error
		return fmt.Errorf("error creating ffmpeg stdout pipe: %w", err)
	}

	buffer := bufio.NewReaderSize(ffmpegout, 16384)
	err = ffmpeg.Start()
	if err != nil {
		// wrap error
		return fmt.Errorf("error starting ffmpeg: %w", err)
	}

	vc.Speaking(true)
	defer vc.Speaking(false)

	var send chan []int16
	if send == nil {
		send = make(chan []int16, 2)
	}

	go sendPCM(vc, send)
	for {
		audioBuffer := make([]int16, FRAME_SIZE*CHANNELS)
		err = binary.Read(buffer, binary.LittleEndian, &audioBuffer)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
		send <- audioBuffer
	}

	return nil
}

func getStreamURL(youtubeURL string) (string, error) {
	cmd := exec.Command("yt-dlp", "-x", "-f", "bestaudio", "-g", youtubeURL)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func sendPCM(voice *discordgo.VoiceConnection, pcm <-chan []int16) {
	encoder, err := gopus.NewEncoder(FRAME_RATE, CHANNELS, gopus.Audio)
	if err != nil {
		fmt.Println("NewEncoder error,", err)
		return
	}

	for {
		receive, ok := <-pcm
		if !ok {
			fmt.Println("PCM channel closed")
			return
		}
		opus, err := encoder.Encode(receive, FRAME_SIZE, MAX_BYTES)
		if err != nil {
			fmt.Println("Encoding error,", err)
			return
		}
		voice.OpusSend <- opus
	}
}
