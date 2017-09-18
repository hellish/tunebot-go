package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"

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

	bot.Debug = false

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

		go HandleVideo(update.Message.Chat.ID, update.Message.MessageID, repo, "medium", "video/mp4", update.Message.Text, bot)
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
func HandleVideo(cid int64, msgid int, repo string, quality string, extension string, url string, bot *tgbotapi.BotAPI) {
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

	ConvertAndServeVideo(cid, msgid, repo, HashString(vl.Title+quality+extension), quality, extension, vl, bot)
}

// ConvertAndServeVideo Downloads video from youtube and sends it to bot
func ConvertAndServeVideo(cid int64, msgid int, repo string, filename string, quality string, extension string, vl yt.VideoList, bot *tgbotapi.BotAPI) {
	fmt.Printf("downloading to %s/%s [quality=%s] [extension=%s]\n", repo, filename, quality, extension)

	SendToBot(cid, msgid, "Downloading video "+vl.Title, bot)
	err := vl.Download(repo, filename, quality, extension)
	if err != nil {
		fmt.Printf("error downloading video %v\n", err)
		SendToBot(cid, msgid, "Error downloading video "+vl.Title, bot)
		return
	}

	fmt.Printf("finished downloading in repository %v\n", repo)

	source := repo + "/" + filename
	target := source + ".mp3"

	//convert to mp3 here
	SendToBot(cid, msgid, "Converting video "+vl.Title+" to mp3", bot)
	fmt.Printf("converting to %s\n", target)
	cmd := exec.Command("ffmpeg", "-i", source, "-f", "mp3", target)
	if _, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("ffmpeg failed: %s", err)
		SendToBot(cid, msgid, "Error converting file "+vl.Title+" to mp3", bot)
		DeleteDownloadedFile(source)
		DeleteDownloadedFile(target)
		return
	}

	SendToBot(cid, msgid, "Uploading "+vl.Title+".mp3 to telegram", bot)
	fmt.Printf("uploading to telegram %s\n", target)
	msg := tgbotapi.NewAudioUpload(cid, target)
	msg.Title = vl.Title
	msg.Caption = vl.Title
	_, err = bot.Send(msg)
	if err != nil {
		fmt.Printf("error file upload %v\n", err)
		SendToBot(cid, msgid, "Error uploading file "+vl.Title, bot)
		DeleteDownloadedFile(source)
		DeleteDownloadedFile(target)
		return
	}

	fmt.Printf("video uploaded %s\n", vl.Title)

	DeleteDownloadedFile(source)
	DeleteDownloadedFile(target)

	fmt.Printf("video success handled %s\n", vl.Title)
}

// HashString MD5 hash of a string
func HashString(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
