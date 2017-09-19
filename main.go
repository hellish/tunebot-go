package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yt "github.com/KeluDiao/gotube/api"
	"github.com/goware/urlx"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

func main() {
	fmt.Println("running tunebot")

	repo := os.Getenv("YOUTUBE_CACHE_FOLDER")
	fmt.Printf("repository setup %s\n", repo)

	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	if telegramToken == "" {
		fmt.Print("telegram token is missing\n")
		os.Exit(1)
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		fmt.Printf("error creating bot %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("authorized on account %s\n", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	fmt.Println("listening for new messages...")

	for update := range updates {
		if update.Message == nil {
			continue
		}

		fmt.Printf("received message from %s\n", update.Message.From)

		go HandleVideo(update.Message.Chat.ID, update.Message.MessageID, repo, update.Message.Text, bot)
	}

}

// SendToBot sends to bot
func SendToBot(cid int64, msgid int, message string, bot *tgbotapi.BotAPI) {
	msg := tgbotapi.NewMessage(cid, message)
	msg.ReplyToMessageID = msgid
	_, err := bot.Send(msg)
	if err != nil {
		fmt.Printf("error sending message %v\n", err)
	}
}

// DeleteDownloadedFile deletes from Os
func DeleteDownloadedFile(path string) {
	fmt.Printf("removing %s\n", path)
	err := os.Remove(path)
	if err != nil {
		fmt.Printf("failed to remove %s - %v\n", path, err)
	}
}

// CheckIfYouTubeURL checks if given url contains n id
func CheckIfYouTubeURL(url string) error {
	u, err := urlx.Parse(url)
	if err != nil {
		return err
	}

	if u.Host != "www.youtube.com" {
		return fmt.Errorf("invalid host %s", u.Host)
	}

	params := u.Query()
	if len(params) == 0 {
		return fmt.Errorf("invalid url %s", url)
	}

	_, ok := params["v"]
	if !ok {
		return fmt.Errorf("url %s does not contain video id", url)
	}

	fmt.Printf("video id identified %v\n", params["v"])

	return nil
}

// HandleVideo Checks if video is valid and processes it
func HandleVideo(cid int64, msgid int, repo string, url string, bot *tgbotapi.BotAPI) {
	fmt.Printf("processing url %s\n", url)

	err := CheckIfYouTubeURL(url)
	if err != nil {
		fmt.Printf("error checking youtube %s - %v\n", url, err)
		SendToBot(cid, msgid, "Invalid url", bot)
		return
	}

	vl, err := yt.GetVideoListFromUrl(url)

	if err != nil {
		fmt.Printf("error getting video list from url %s - %v\n", url, err)
		return
	}

	ConvertAndServeVideo(cid, msgid, repo, url, vl, bot)
}

// ConvertAndServeVideo Downloads video from youtube and sends it to bot
func ConvertAndServeVideo(cid int64, msgid int, repo string, url string, vl yt.VideoList, bot *tgbotapi.BotAPI) {
	fmt.Printf("converting and serving %s\n", url)

	cmd := exec.Command("youtube-dl", url, "--get-filename", "--no-warnings")
	bfilename, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("error fetching video details %v\n", err)
		SendToBot(cid, msgid, "Error fetching details for "+vl.Title, bot)
		return
	}

	filename := string(bfilename[:])
	fmt.Printf("output filename found %s", filename)
	source := strings.Trim(repo+"/"+filename, "\n")
	extension := strings.Trim(filepath.Ext(source), "\n")

	SendToBot(cid, msgid, "Downloading video "+vl.Title, bot)
	mp3 := strings.TrimSuffix(source, extension) + ".mp3"
	cmd = exec.Command("youtube-dl", "--extract-audio", "--audio-format", "mp3", "--audio-quality", "3", "--prefer-ffmpeg", url, "-o", source, "--no-warnings")
	_, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("error downloading video %v\n", err)
		SendToBot(cid, msgid, "Error downloading video "+vl.Title, bot)
		DeleteDownloadedFile(source)
		DeleteDownloadedFile(mp3)
		return
	}

	fmt.Printf("mp3 downloaded %s\n", mp3)

	SendToBot(cid, msgid, "Uploading "+vl.Title+" to telegram", bot)
	fmt.Printf("uploading to telegram %s\n", mp3)
	msg := tgbotapi.NewAudioUpload(cid, mp3)
	msg.Title = vl.Title
	msg.Caption = vl.Title
	_, err = bot.Send(msg)
	if err != nil {
		fmt.Printf("error file upload %v\n", err)
		SendToBot(cid, msgid, "Error uploading file "+vl.Title, bot)
		DeleteDownloadedFile(mp3)
		return
	}

	fmt.Printf("video uploaded %s\n", vl.Title)

	DeleteDownloadedFile(mp3)

	fmt.Printf("video success handled %s\n", vl.Title)
}
