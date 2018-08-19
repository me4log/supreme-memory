package libs

import (
	"net/http"
	"time"
	"os"
	"io"
	"log"
	"github.com/djimenez/iconv-go"
)

func DownloadAndSave(url string, file string, charset string) (error) {

	client := http.Client{Timeout: 30 * time.Second}
	res, err := client.Get(url)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return err;
	}

	f, err := os.Create(file)
	if err != nil {
		return err;
	}
	defer f.Close()

	if charset != "" {
		utfBody, err := iconv.NewReader(res.Body, charset, "utf-8")
		if err != nil {
			return err
		}
		io.Copy(f, utfBody)
	} else {
		io.Copy(f, res.Body)
	}

	return nil
}

func DownloadAndSaveChan(url string, file string, c chan string, e chan error, charset string) {
	err := DownloadAndSave(url, file, charset)
	if err != nil {
		e <- err
		return
	}
	c <- url
}

func DownloadAndSaveSem(url string, file string, sem chan struct{}, charset string) {
	if err := DownloadAndSave(url, file, charset); err != nil {
		log.Println(err)
		return
	}
	<-sem
}
