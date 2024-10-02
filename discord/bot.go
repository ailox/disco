package discord

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var buffer = make([][]byte, 0)

// Declare dcaExecutablePath as a global variable within the discord package
var dcaExecutablePath = "/home/fabio/go/bin/dca"

var cmdid string

func Disco(token string) {
	if token == "" {
		fmt.Println("No token provided. please use -t <bot token>")
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
	s.UpdateGameStatus(0, "/play")

	// register the /play command
	_, err := s.ApplicationCommandCreate(s.State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        "play",
		Description: "Plays a song from a YouTube URL in your current voice channel",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "The YouTube URL of the song to play",
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
}

func commandCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {

	if i.ApplicationCommandData().Name == "play" {
		// Extract YouTube URL from interaction data
		youtubeURL := i.ApplicationCommandData().Options[0].StringValue()

		// Download and convert video to .dca
		dcaFilePath, err := downloadAndConvertToDCA(youtubeURL)
		if err != nil {
			// Handle error, send error message to user
			fmt.Println("Error downloading and converting:", err)
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error playing the requested song.",
				},
			})
			return
		}

		// Load .dca file into buffer
		err = loadSound(dcaFilePath)
		if err != nil {
			// Handle error, send error message to user
			fmt.Println("Error loading sound:", err)
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error playing the requested song.",
				},
			})
			return
		}

		// Find user's voice channel and play the sound
		err = playSoundInUserChannel(s, i)
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
				Content: "Playing your requested song! 🎵",
			},
		})
	}
}

// loadSound attempts to load an encoded sound file from disk.
func loadSound(filePath string) error {
	// Reset the buffer before loading a new sound
	buffer = make([][]byte, 0)
	file, err := os.Open(filePath)
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
	}
}

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

func downloadAndConvertToDCA(youtubeURL string) (string, error) {
	// Define the output file path
	dcaFilePath := "temp.dca"

	// Construct the yt-dlp command to download the best audio stream
	ydlCmd := exec.Command("yt-dlp", "-f", "bestaudio", "-o", "-", youtubeURL) // Output to stdout ('-')

	// Construct the dca command without encoding options
	dcaCmd := exec.Command(dcaExecutablePath, dcaFilePath)

	// Pipe yt-dlp output to dca input
	dcaCmd.Stdin, _ = ydlCmd.StdoutPipe()

	// Execute the commands
	err := ydlCmd.Start()
	if err != nil {
		return "", fmt.Errorf("error starting yt-dlp: %v", err)
	}

	output, err := dcaCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error converting to DCA: %v, output: %s", err, output)
	}

	fmt.Println("dca output:", string(output)) // Print dca output for debugging

	// Wait for yt-dlp to finish
	err = ydlCmd.Wait()
	if err != nil {
		return "", fmt.Errorf("error waiting for yt-dlp: %v", err)
	}

	// Add a small delay to ensure the file is fully written
	time.Sleep(500 * time.Millisecond)

	return dcaFilePath, nil
}

func playSoundInUserChannel(s *discordgo.Session, i *discordgo.InteractionCreate) (err error) {
	// Find the channel that the message came from.
	c, err := s.State.Channel(i.ChannelID)
	if err != nil {
		// Could not find channel.
		return err
	}

	// Find the guild for that channel.
	g, err := s.State.Guild(c.GuildID)
	if err != nil {
		// Could not find guild.
		return err
	}

	// Look for the message sender in that guild's current voice states.
	for _, vs := range g.VoiceStates {
		if vs.UserID == i.Member.User.ID {
			return playSound(s, g.ID, vs.ChannelID)
		}
	}

	return fmt.Errorf("user not in a voice channel")
}
