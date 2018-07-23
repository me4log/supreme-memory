package autofanatik

import (
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	SipeMapUrl = "http://autofanatik.ru/sitemap.xml"
)

type SiteMapUrl struct {
	Url string `xml:"loc"`
}

type SiteMapUrls struct {
	XMLName xml.Name      `xml:"urlset"`
	Urls    []*SiteMapUrl `xml:"url"`
}

func getSiteMap() {

	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(SipeMapUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	f, err := os.Create("autofanatik/data/sitemap.xml")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	io.Copy(f, res.Body)
}

func Run() {
	getSiteMap()
}
