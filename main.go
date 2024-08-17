package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const MAX_MESSAGE_SIZE = 2000
const MAX_TOP_MESSAGES = 4

func Login() (*discordgo.Session, error) {
	token := os.Getenv("DISCORD_TOKEN")
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Discord: %v", err)
	}

	return discord, nil
}

func SendDiscordMessage(channelID string, content string) error {
	discord, err := Login()
	if err != nil {
		return fmt.Errorf("could not send message: %v", err)
	}

	const MAX_MESSAGE_SIZE_ADJUSTED = MAX_MESSAGE_SIZE - 6

	i := 0
	for (i+1)*MAX_MESSAGE_SIZE_ADJUSTED <= len(content) {
		_, err = discord.ChannelMessageSend(channelID, "```" + (content[(i*MAX_MESSAGE_SIZE_ADJUSTED):((i+1)*MAX_MESSAGE_SIZE_ADJUSTED)]) + "```", func(_ *discordgo.RequestConfig) {})
		if err != nil {
			return err
		}
		i++
	}

	lastChunkLength := len(content) - i*MAX_MESSAGE_SIZE_ADJUSTED
	if lastChunkLength != 0 {
		_, err = discord.ChannelMessageSend(channelID, "```" + (content[(i*MAX_MESSAGE_SIZE_ADJUSTED):]) + "```", func(_ *discordgo.RequestConfig) {})
	}

	return err
}

func GetSensorsOrDie() string {
	cmd := exec.Command("sensors", "-j")

	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Could not grab sensors output: %v", err)
	}

	return string(output)
}

func GetTop() (string, error) {
	cmd := exec.Command("top", "-b", "-n", "1", "-H", "--sort-override", "%CPU")

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

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

func main() {
	channelID := flag.String("channelID", "", "Channel ID to send warnings.")
	limit := flag.Float64("limit", 0, "Sets the threshold. Shall be > 0.")
	flag.Parse()

	if *channelID == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *limit == 0 {
		flag.Usage()
		os.Exit(1)
	}

	var sensorsJson map[string]any
	sensorsOutput := GetSensorsOrDie()
	err := json.Unmarshal([]byte(sensorsOutput), &sensorsJson)
	if err != nil {
		log.Fatal("wtf wrong JSON in sensors.")
	}

	if TraverseSensors(&sensorsJson, *limit) {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = fmt.Sprintf("<COULDN'T GET HOSTNAME: %v>", err)
		}

		topOutput, err := GetTop()
		if err != nil {
			topOutput = fmt.Sprintf("<COULDN'T GET TOP OUTPUT: %v>", err)
		}
		topOutput = topOutput[:MAX_MESSAGE_SIZE*MAX_TOP_MESSAGES]

		message := fmt.Sprintf("\nWarning for machine %v: high temperature.\n\nSensors output:\n\n%v\n\nTop output:\n\n%v\n\n", hostname, sensorsOutput, topOutput)

		err = SendDiscordMessage(*channelID, message)
		if err != nil {
			log.Fatalf("WARNING: HIGH TEMPERATURE DETECTED YET UNABLE TO SEND MESSAGE: %v", err)
		}
	}
}
