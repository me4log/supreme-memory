package vseinstrumenty

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DataPath   = "grabers/vseinstrumenty/data/"
	SiteMapURL = "http://www.vseinstrumenti.ru/sitemap.xml"
	FilesCount = 13
	Template1  = "http://www.vseinstrumenti.ru/instrument/shurupoverty/"
	Template2  = "http://www.vseinstrumenti.ru/instrument/perforatory/"
	Template3  = "http://www.vseinstrumenti.ru/instrument/dreli/"
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
	ShortName    string                  `xml:"shortName"`
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

func downloadAndSaveXML(url string, fileIndex string, wg *sync.WaitGroup) {

	defer wg.Done()

	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	fileName := fileIndex + ".xml"

	f, err := os.Create(DataPath + fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	io.Copy(f, res.Body)
	log.Println(fileName)
}

func StepOne() {

	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(SiteMapURL)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	siteMap := new(SiteMapIndex)
	xml.NewDecoder(res.Body).Decode(siteMap)

	f, err := os.Create(DataPath + "/sitemap.xml")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	xml.NewEncoder(f).Encode(siteMap)

	var wg sync.WaitGroup
	for index, url := range siteMap.Maps {
		fileIndex := strconv.Itoa(index)
		log.Println(fileIndex + ":" + url.Url)
		wg.Add(1)
		go downloadAndSaveXML(url.Url, fileIndex, &wg)
	}
	wg.Wait()
}

func getLinks(fileIndex int, c chan []string) {

	links := make([]string, 0)
	f, err := os.Open(DataPath + strconv.Itoa(fileIndex) + ".xml")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	urlset := new(SiteMapUrls)
	xml.NewDecoder(f).Decode(urlset)

	for _, url := range urlset.Urls {
		if strings.Contains(url.Url, Template1) ||
			strings.Contains(url.Url, Template2) ||
			strings.Contains(url.Url, Template3) {
			links = append(links, url.Url)
		}
	}

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
	writeLines(links, DataPath+"/links.txt")
}

func getAndSavePage(url string, fileName string) {
	client := http.Client{Timeout: 10 * time.Second}

	res, err := client.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	f, err := os.Create(DataPath + "/pages/" + fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	io.Copy(f, res.Body)
	log.Println(fileName + ": " + url)
}

func StepThree() {
	links, err := readLines(DataPath + "/links.txt")
	if err != nil {
		log.Fatal(err)
	}

	for index, url := range links {
		getAndSavePage(url, strconv.Itoa(index)+".html")
	}

}

func StepFour() {

	catalog := new(Catalog)

	files, err := ioutil.ReadDir(DataPath + "/pages")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		log.Println(file.Name())

		f, err := os.Open(DataPath + "/pages/" + file.Name())
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		doc, err := goquery.NewDocumentFromReader(f)
		if err != nil {
			log.Fatal(err)
		}

		chars := doc.Find("#allCharacteristics .thValueBlock")
		if len(chars.Nodes) == 0 {
			continue
		}

		item := new(CatalogItem)

		item.Name = strings.TrimSpace(strings.Replace(doc.Find("#card-h1-reload-new").Text(), "\n", "", -1))
		item.Description = strings.TrimSpace(strings.Replace(doc.Find("[itemprop=\"description\"] p").Text(), "\n", "", -1))
		item.ShortName = strings.TrimSpace(strings.Replace(strings.Replace(doc.Find("#cardVendorSclonenie13").Text(), "\n", "", -1), "Технические характеристики", "", -1))

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

	rf, err := os.Create(DataPath + "/catalog.xml")
	if err != nil {
		log.Fatal(err)
	}
	defer rf.Close()

	xml.NewEncoder(rf).Encode(catalog)
}

func Run() {
	log.Println("Start")
	StepOne()
	StepTwo()
	StepThree()
	StepFour()
	log.Println("Finish")
}
