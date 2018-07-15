package main

import (
	"encoding/xml"
	"net/http"
	"time"
	"log"
	"os"
	"sync"
	"io"
	"strconv"
	"strings"
	"bufio"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
)

const (
	SiteMapURL = "http://www.vseinstrumenti.ru/sitemap.xml"
	FilesCount = 17
	Template1  = "http://www.vseinstrumenti.ru/instrument/shurupoverty/akkumulyatornye_dreli-shurupoverty/"
	Template2  = "http://www.vseinstrumenti.ru/instrument/perforatory/"
)

type CatalogItemMeasure struct {
	XMLName xml.Name `xml:"measurement"`
	Key     string   `xml:"key"`
	Value   string   `xml:"value"`
}

type CatalogItemAttribute struct {
	XMLName xml.Name `xml:"attribute"`
	Key     string   `xml:"key"`
	Value   string   `xml:"value"`
}

type CatalogItem struct {
	XMLName      xml.Name                `xml:"catalogItem"`
	Name         string                  `xml:"name"`
	Description  string                  `xml:"description"`
	Attributes   []*CatalogItemAttribute `xml:"attributes>attribute"`
	Equipment    []string                `xml:"equipments>equipment"`
	Measurements []*CatalogItemMeasure   `xml:"measurements>measurement"`
}

type Catalog struct {
	XMLName xml.Name       `xml:"catalog"`
	Items   []*CatalogItem `xml:"items"`
}

type SiteMapItem struct {
	Url string `xml:"loc"`
}

type SiteMapIndex struct {
	XMLName xml.Name       `xml:"sitemapindex"`
	Maps    []*SiteMapItem `xml:"sitemap"`
}

type SiteMapUrl struct {
	Url string `xml:"loc"`
}

type SiteMapUrls struct {
	XMLName xml.Name      `xml:"urlset"`
	Urls    []*SiteMapUrl `xml:"url"`
}

var mu sync.Mutex
var fileIndex = 0

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}

func downloadAndSaveXML(url string, wg *sync.WaitGroup) {

	defer wg.Done()

	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(url)
	defer res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	mu.Lock()
	fileIndex++
	fileName := strconv.Itoa(fileIndex) + ".xml"
	f, err := os.Create("data/" + fileName)
	mu.Unlock()

	defer f.Close()
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(f, res.Body)

	log.Println(fileName)
}

func StepOne() {

	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(SiteMapURL)
	defer res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	siteMap := new(SiteMapIndex)
	xml.NewDecoder(res.Body).Decode(siteMap)

	f, err := os.Create("data/sitemap.xml")
	defer f.Close()
	if err != nil {
		log.Fatal(err)
	}

	xml.NewEncoder(f).Encode(siteMap)

	var wg sync.WaitGroup
	for index, url := range siteMap.Maps {
		log.Println(strconv.Itoa(index) + ":" + url.Url)
		wg.Add(1)
		go downloadAndSaveXML(url.Url, &wg)
	}
	wg.Wait()
}

func getLinks(fileIndex int, c chan []string) {

	links := make([]string, 0)
	f, err := os.Open("data/" + strconv.Itoa(fileIndex) + ".xml")
	defer f.Close()
	if err != nil {
		log.Fatal(err)
	}

	urlset := new(SiteMapUrls)
	xml.NewDecoder(f).Decode(urlset)

	for _, url := range urlset.Urls {
		if strings.Contains(url.Url, Template1) || strings.Contains(url.Url, Template2) {
			links = append(links, url.Url)
		}
	}
	log.Println(fileIndex)
	log.Println(len(links))
	c <- links

}

func StepTwo() {
	c := make(chan []string)
	for i := 1; i <= FilesCount; i++ {
		go getLinks(i, c)
	}

	links := make([]string, 0)
	for i := 1; i <= FilesCount; i++ {
		for _, link := range <-c {
			links = append(links, link)
		}
	}
	close(c)
	writeLines(links, "data/links.txt")
}

func getAndSavePage(url string, fileName string) {
	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(url)
	defer res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("data/pages/" + fileName)
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(f, res.Body)
	log.Println(fileName + ": " + url)
}

func StepThree() {
	links, err := readLines("data/links.txt")
	if err != nil {
		log.Fatal(err)
	}

	for index, url := range links {
		getAndSavePage(url, strconv.Itoa(index)+".html")
	}

}

func StepFour() {

	catalog := new(Catalog)

	files, err := ioutil.ReadDir("data/pages")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		log.Println(file.Name())
		f, err := os.Open("data/pages/" + file.Name())
		defer f.Close()
		if err != nil {
			log.Fatal(err)
		}

		doc, err := goquery.NewDocumentFromReader(f)
		if err != nil {
			log.Fatal(err)
		}

		chars := doc.Find("#allCharacteristics .thValueBlock")
		if len(chars.Nodes) == 0 {
			continue
		}

		item := new(CatalogItem)
		item.Name = strings.Replace(doc.Find("#card-h1-reload-new").Text(), "\n", "", -1)
		item.Description = strings.Replace(doc.Find("[itemprop=\"description\"] p").Text(), "\n", "", -1)
		chars.Each(func(i1 int, s1 *goquery.Selection) {
			attribute := new(CatalogItemAttribute)
			attribute.Key = s1.Find(".thName").Text()
			attribute.Value = s1.Find(".thValue").Text()
			item.Attributes = append(item.Attributes, attribute)
		})

		measures := strings.Replace(strings.Replace(doc.Find("#vgh-block div").Text(), "\n", "", 1), "\n", "#", 2)
		measureList := strings.Split(measures, "#")

		for _, m := range measureList {
			measure := new(CatalogItemMeasure)
			m0 := strings.Split(m, ":")
			measure.Key = strings.Replace(m0[0], "\n", "", -1)
			if len(m0) > 1 {
				measure.Value = strings.Replace(m0[1], "\n", "", -1)
			} else {
				measure.Value = ""
			}
			item.Measurements = append(item.Measurements, measure)
		}

		doc.Find(".complect li").Each(func(i1 int, s1 *goquery.Selection) {
			item.Equipment = append(item.Equipment, s1.Text())
		})

		catalog.Items = append(catalog.Items, item)
	}

	rf, err := os.Create("data/catalog.xml")
	defer rf.Close()
	if err != nil {
		log.Fatal(err)
	}
	xml.NewEncoder(rf).Encode(catalog)
}

func main() {
	log.Println("Start")
	//stepOne()
	//StepTwo()
	//StepThree()
	StepFour()
	log.Println("Finish")
}
