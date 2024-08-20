package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// The maximum size of Discord messages is (as of today) 2000 characters. This
// constant is used in SendDiscordMessage to send a single block of text as
// multiple separated messages.
const MAX_MESSAGE_SIZE = 2000

// The top output is usually huge, so this limits the amount of output to send
// by multiplying by MAX_MESSAGE_SIZE.
const MAX_TOP_MESSAGES = 4

// Tries to login to Discord.
func Login() (*discordgo.Session, error) {
	token := os.Getenv("DISCORD_TOKEN")
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Discord: %v", err)
	}

	return discord, nil
}

// Sends (possibly multiple) messages, by separating the content in
// MAX_MESSAGE_SIZE characters chunks.
func SendDiscordMessage(discord *discordgo.Session, channelID string, content string) (err error) {
	// We adjust the size because the message will be wrapped in ```.
	const MAX_MESSAGE_SIZE_ADJUSTED = MAX_MESSAGE_SIZE - 6

	// Loops sending a maximum of 2000 characters each time.
	i := 0
	for (i+1)*MAX_MESSAGE_SIZE_ADJUSTED <= len(content) {
		_, err = discord.ChannelMessageSend(channelID, "```"+(content[(i*MAX_MESSAGE_SIZE_ADJUSTED):((i+1)*MAX_MESSAGE_SIZE_ADJUSTED)])+"```", func(_ *discordgo.RequestConfig) {})
		if err != nil {
			// Implicitly return err.
			return
		}
		i++
	}

	// (Possibly) Sends the remaining characters.
	lastChunkLength := len(content) - i*MAX_MESSAGE_SIZE_ADJUSTED
	if lastChunkLength != 0 {
		_, err = discord.ChannelMessageSend(channelID, "```"+(content[(i*MAX_MESSAGE_SIZE_ADJUSTED):])+"```", func(_ *discordgo.RequestConfig) {})
	}

	// Implicitly return err.
	return
}

// Gets the output of sensors in JSON format.
func GetSensors() (string, error) {
	cmd := exec.Command("sensors", "-j")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not grab sensors output: %v", err)
	}

	return string(output), nil
}

// Gets the output of top.
func GetTop() (string, error) {
	// -b is batch mode, so it stops after some iterations.
	// -n 1 sets the number of iterations to a single one.
	// -H shows threads.
	// --sort-override %CPU sorts the output by CPU usage.
	cmd := exec.Command("top", "-b", "-n", "1", "-H", "--sort-override", "%CPU")

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Recursively searches for the temperature sensors, returning true if there's a
// sensors above the limit.
func TraverseSensors(sensors *map[string]any, limit float64) bool {
	for k, v := range *sensors {
		children, isMap := v.(map[string]any)
		if !isMap {
			continue
		}
		if strings.HasPrefix(k, "temp") {
			valueAsAny, ok := children[k+"_input"]
			if !ok {
				continue
			}
			value, ok := valueAsAny.(float64)
			if !ok {
				continue
			}
			if value > limit {
				return true
			}
		} else {
			if TraverseSensors(&children, limit) {
				return true
			}
		}
	}

	return false
}

func Run(channelID string, hostname string, limit float64) (bool, string, error) {
	var sensorsJson map[string]any
	sensorsOutput, err := GetSensors()
	if err != nil {
		return false, "", fmt.Errorf("cannot get sensors output: %v", err)
	}

	err = json.Unmarshal([]byte(sensorsOutput), &sensorsJson)
	if err != nil {
		return false, "", fmt.Errorf("wrong JSON in sensors output: %v", err)
	}

	if TraverseSensors(&sensorsJson, limit) {
		topOutput, err := GetTop()
		if err != nil {
			topOutput = fmt.Sprintf("<COULDN'T GET TOP OUTPUT: %v>", err)
		}
		topOutput = topOutput[:MAX_MESSAGE_SIZE*MAX_TOP_MESSAGES]

		message := fmt.Sprintf("\nWarning for machine %v: high temperature.\n\nSensors output:\n\n%v\n\nTop output:\n\n%v\n\n", hostname, sensorsOutput, topOutput)

		return true, message, nil
	}

	return false, "", nil
}

func RunAndSendMessage(discord *discordgo.Session, channelID string, hostname string, limit float64) {
	warn, message, err := Run(channelID, hostname, limit)
	if !warn && err != nil {
		message = fmt.Sprintf("Warning for machine %v: ERROR during check: %v", hostname, err)
	}

	err = SendDiscordMessage(discord, channelID, message)
	if err != nil {
		log.Printf("WARNING: CANNOT SEND MESSAGE TO DISCORD. CONTENT: %v", message)
	}
}

func main() {
	channelID := flag.String("channelID", "", "Channel ID to send warnings.")
	limit := flag.Float64("limit", 0, "Sets the threshold. Shall be > 0.")
	sleepTime := flag.Uint64("sleepTime", 0, "Sets the sleep time in seconds (to daemonize). If 0, don't daemonize.")
	flag.Parse()

	if *channelID == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *limit == 0 {
		flag.Usage()
		os.Exit(1)
	}

	discord, err := Login()
	if err != nil {
		log.Fatalf("Cannot login to Discord: %v", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = fmt.Sprintf("<COULDN'T GET HOSTNAME: %v>", err)
	}

	if *sleepTime == 0 {
		RunAndSendMessage(discord, *channelID, hostname, *limit)
	} else {
		for {
			RunAndSendMessage(discord, *channelID, hostname, *limit)
			time.Sleep(time.Duration(*sleepTime) * time.Second)
		}
	}
}
