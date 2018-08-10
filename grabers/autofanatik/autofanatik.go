package autofanatik

import (
	"encoding/xml"
	"net/http"
	"time"
	"os"
	"io"
	"log"
	"strconv"
	"github.com/PuerkitoBio/goquery"
	"fmt"
	"path"
	"io/ioutil"
	"strings"
	"sync"
)

const (
	BaseUrl            = "http://autofanatik.ru"
	SipeMapUrl         = "http://autofanatik.ru/sitemap.xml"
	DataPath           = "grabers/autofanatik/data/"
	PagesDataPath      = DataPath + "pages/"
	ImagesDataPath     = DataPath + "images/"
	SiteMapPath        = DataPath + "sitemap.xml"
	CatalogPath        = DataPath + "/catalog.xml"
	Catalog0Path       = DataPath + "/catalog0.xml"
	Catalog1Path       = DataPath + "/catalog1.xml"
	Catalog2Path       = DataPath + "/catalog2.xml"
	Catalog3Path       = DataPath + "/catalog3.xml"
	CatalogUnknownPath = DataPath + "/catalog4.xml"
)

const (
	// Необходимо отрезать первый символ
	ENCODE_TYPE_1 = "1"
	// Совпадение с артикулом по N-1 символам
	ENCODE_TYPE_2 = "2"
	// Полное совпадение с артикулом
	ENCODE_TYPE_3 = "3"
	// Неизвестно
	ENCODE_TYPE_UNKNOWN = "Unknown"
)

type FixedUrl struct {
	SourceUrl  string `xml:"source"`
	FixedUrl   string `xml:"fixed"`
	EncodeType string `xml:"encodeType"`
	FileName   string `xml:"image"`
}

type CatalogItem struct {
	XMLName     xml.Name    `xml:"item"`
	Name        string      `xml:"name"`
	Collection  string      `xml:"collection"`
	Description string      `xml:"description"`
	Article     string      `xml:"alticle"`
	Price       string      `xml:"price"`
	Urls        []string    `xml:"urls>url"`
	FixedUrls   []*FixedUrl `xml:"fixedUrls>url"`
}

type Catalog struct {
	XMLName xml.Name       `xml:"catalog"`
	Items   []*CatalogItem `xml:"items>item"`
}

type SiteMapUrl struct {
	Url string `xml:"loc"`
}

type SiteMapUrls struct {
	XMLName xml.Name      `xml:"urlset"`
	Urls    []*SiteMapUrl `xml:"url"`
}

func downloadAndSave(url string, file string) (error) {

	client := http.Client{Timeout: 10 * time.Second}
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

	io.Copy(f, res.Body)
	return nil
}

func downloadAndSaveChan(url string, file string, c chan struct{}, e chan error) {
	err := downloadAndSave(url, file)
	if err != nil {
		e <- err
		return
	}
	c <- struct{}{}
}

func getSiteMap() (error) {
	return downloadAndSave(SipeMapUrl, SiteMapPath)
}

func getPages() (error) {

	f, err := os.Open(SiteMapPath);
	if err != nil {
		return err
	}
	defer f.Close()

	siteMap := new(SiteMapUrls)
	xml.NewDecoder(f).Decode(siteMap)

	log.Println("Start downloads " + strconv.Itoa(len(siteMap.Urls)) + " files")
	for index, url := range siteMap.Urls {
		fileName := PagesDataPath + "page" + strconv.Itoa(index) + ".html"
		log.Println("** " + strconv.Itoa(index) + ".html :" + url.Url)
		if err := downloadAndSave(url.Url, fileName); err != nil {
			log.Println(err);
		}
	}

	return nil
}

func parsePage(filename string) (*CatalogItem, error) {

	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.Println(f.Name())

	item := new(CatalogItem)
	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		log.Fatal(err)
	}

	title := doc.Find(".good_title h1")
	if len(title.Nodes) == 0 {
		return nil, fmt.Errorf("no title")
	}
	item.Name = title.Text()

	description := doc.Find(".description p")
	if len(description.Nodes) > 0 {
		item.Description = description.Text()
	}

	collection := doc.Find(".good_collection")
	if len(collection.Nodes) > 0 {
		item.Collection = collection.Text()
	}

	article := doc.Find("td.good_text div #for_artikul")
	if len(article.Nodes) > 0 {
		item.Article = article.Text()
	}

	price := doc.Find("td.good_text .price")
	if len(price.Nodes) > 0 {
		item.Price = price.Text()
	}

	item.Urls = make([]string, 0)
	image, ok := doc.Find("td.good_img a").Attr("href")
	if (ok) {
		item.Urls = append(item.Urls, BaseUrl+image)
	}

	addImages := doc.Find("td.good_img div.add_img_previews a")
	if len(addImages.Nodes) > 0 {
		for _, node := range addImages.Nodes {
			for _, attr := range node.Attr {
				if attr.Key != "href" {
					continue
				}
				d, f := path.Split(attr.Val)
				imgUrl := BaseUrl + d + f
				item.Urls = append(item.Urls, imgUrl)
			}
		}
	}

	return item, nil
}

func parsePages() (*Catalog, error) {

	d, err := ioutil.ReadDir(PagesDataPath)
	if err != nil {
		return nil, err
	}

	catalog := new(Catalog)
	catalog.Items = make([]*CatalogItem, 0)

	for _, item := range d {
		if item.IsDir() {
			continue
		}
		catalogItem, err := parsePage(PagesDataPath + item.Name());
		if err != nil {
			log.Println(err)
		}
		if catalogItem != nil {
			catalog.Items = append(catalog.Items, catalogItem)
		}
	}
	return catalog, nil
}

func saveCatalog(catalog *Catalog, filename string) (error) {
	sf, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer sf.Close()
	return xml.NewEncoder(sf).Encode(catalog)
}

func openCatalog(filename string) (*Catalog, error) {
	catalog := new(Catalog)
	rf, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer rf.Close()
	xml.NewDecoder(rf).Decode(catalog)
	return catalog, nil
}

func findImages() (error) {
	catalog, err := openCatalog(CatalogPath)
	if err != nil {
		return err
	}

	catalog0 := new(Catalog)
	catalog1 := new(Catalog)
	catalog2 := new(Catalog)
	catalog3 := new(Catalog)
	catalogUnknown := new(Catalog)

	catalog0.Items = make([]*CatalogItem, 0)
	catalog1.Items = make([]*CatalogItem, 0)
	catalog2.Items = make([]*CatalogItem, 0)
	catalog3.Items = make([]*CatalogItem, 0)
	catalogUnknown.Items = make([]*CatalogItem, 0)

	for _, item := range catalog.Items {
		item.FixedUrls = make([]*FixedUrl, 0)

		magicNumber := ""
		for i, _ := range item.Article {
			if _, err := strconv.Atoi(item.Article[i : i+1]); err != nil {
				break;
			}
			magicNumber = magicNumber + item.Article[i:i+1]
		}

		for _, url := range item.Urls {
			fixedUrl := new(FixedUrl)
			fixedUrl.SourceUrl = url
			fixedUrl.EncodeType = ENCODE_TYPE_UNKNOWN
			d, f := path.Split(url)

			if strings.Index(f, item.Article) > 0 {
				fixedUrl.EncodeType = ENCODE_TYPE_1
			} else if strings.Index(f, item.Article) == 0 {
				fixedUrl.EncodeType = ENCODE_TYPE_3
			} else if strings.Index(f, item.Article[:len(item.Article)-2]) == 0 {
				fixedUrl.EncodeType = ENCODE_TYPE_2
			}

			if fixedUrl.EncodeType == ENCODE_TYPE_1 {
				fixedUrl.FixedUrl = d + f[strings.Index(f, item.Article):]
			}

			if fixedUrl.EncodeType == ENCODE_TYPE_2 {
				if err != nil {
					log.Println(err)
				}
				f = magicNumber + f[len(magicNumber):]
				fixedUrl.FixedUrl = d + f
			}

			item.FixedUrls = append(item.FixedUrls, fixedUrl)

		}

		if len(item.FixedUrls) > 0 {
			if item.FixedUrls[0].EncodeType == ENCODE_TYPE_1 {
				catalog1.Items = append(catalog1.Items, item)
			}

			if item.FixedUrls[0].EncodeType == ENCODE_TYPE_2 {
				catalog2.Items = append(catalog2.Items, item)
			}

			if item.FixedUrls[0].EncodeType == ENCODE_TYPE_3 {
				catalog3.Items = append(catalog3.Items, item)
			}

			if item.FixedUrls[0].EncodeType == ENCODE_TYPE_UNKNOWN {
				catalogUnknown.Items = append(catalogUnknown.Items, item)
			}
		} else {
			catalog0.Items = append(catalog0.Items, item)
		}
	}

	err = saveCatalog(catalog0, Catalog0Path)
	if err != nil {
		return err
	}
	//err = saveCatalog(catalog1, Catalog1Path)
	//if err != nil {
	//	return err
	//}
	//err = saveCatalog(catalog2, Catalog2Path)
	//if err != nil {
	//	return err
	//}
	err = saveCatalog(catalog3, Catalog3Path)
	if err != nil {
		return err
	}
	err = saveCatalog(catalogUnknown, CatalogUnknownPath)
	if err != nil {
		return err
	}

	return nil
}

var i int = 0

func getNextImageName() (string) {
	var m = sync.Mutex{}
	m.Lock()
	i++
	m.Unlock()
	index := strconv.Itoa(i)
	return strings.Repeat("0", 10-len(index)) + index
}

func downloadCatalogImages(catalog *Catalog, prefix string) {

	dc := make(chan struct{}, 10)
	ec := make(chan error, 10)

	total := len(catalog.Items)
	for index, item := range catalog.Items {

		log.Println(prefix + "(" + strconv.Itoa(index+1) + "/" + strconv.Itoa(total) + ") " + item.Name)

		for _, fixedUrl := range item.FixedUrls {

			_, f := path.Split(fixedUrl.FixedUrl)
			e := path.Ext(f)
			imageName := getNextImageName() + e
			imagePath := ImagesDataPath + imageName

			go downloadAndSaveChan(fixedUrl.FixedUrl, imagePath, dc, ec)

			select {
			case <-dc:
				fixedUrl.FileName = imageName;
			case err := <-ec:
				log.Println(err)
			}
		}
	}

	close(dc)
	close(ec)
}

func downloadImages() (error) {

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		catalog1, err := openCatalog(Catalog1Path)
		if err != nil {
			log.Println(err)
		}
		downloadCatalogImages(catalog1, "C1")
		saveCatalog(catalog1, Catalog1Path)
		wg.Done()
	}()

	go func() {
		catalog2, err := openCatalog(Catalog2Path)
		if err != nil {
			log.Println(err)
		}
		downloadCatalogImages(catalog2, "C2")
		saveCatalog(catalog2, Catalog2Path)
		wg.Done()
	}()

	wg.Wait()
	return nil
}

func Run() {
	//getSiteMap()
	//getPages()

	//catalog, err := parsePages()
	//if err != nil {
	//	log.Print(err)
	//}
	//
	//if catalog != nil {
	//	if err := saveCatalog(catalog, CatalogPath); err != nil {
	//		log.Fatal(err)
	//	}
	//}

	if err := findImages(); err != nil {
		log.Fatal(err)
	}

	//if err := downloadImages(); err != nil {
	//	log.Fatal(err)
	//}
}
