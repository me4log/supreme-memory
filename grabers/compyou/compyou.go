package compyou

import (
	lib "goods.ru/grab-it/libs"
	"encoding/xml"
	"os"
	"log"
	"strconv"
	"io/ioutil"
	"github.com/PuerkitoBio/goquery"
	"io"
	"github.com/djimenez/iconv-go"
	"gopkg.in/cheggaaa/pb.v1"
	"strings"
)

const (
	SipeMapUrl     = "http://compyou.ru/sitemap.xml"
	DataPath       = "/Users/vodolazov/go-data/compyou/"
	PagesDataPath  = DataPath + "pages/"
	ImagesDataPath = DataPath + "images/"
	SiteMapPath    = DataPath + "sitemap.xml"
	CatalogPath    = DataPath + "/catalog.xml"
)

type SiteMapUrl struct {
	Url string `xml:"loc"`
}

type SiteMapUrls struct {
	XMLName xml.Name      `xml:"urlset"`
	Urls    []*SiteMapUrl `xml:"url"`
}

type Attribute struct {
	XMLName xml.Name `xml:"attribute"`
	Key     string   `xml:"key"`
	Value   string   `xml:"value"`
}

type AttributeGroup struct {
	XMLName    xml.Name     `xml:"group"`
	Name       string       `xml:"name"`
	Attributes []*Attribute `xml:"attributes>attribute"`
}

type Image struct {
	Url  string `xml:"url"`
	File string `xml:"file"`
}

type CatalogItem struct {
	XMLName         xml.Name          `xml:"item"`
	Name            string            `xml:"name"`
	AttributeGroups []*AttributeGroup `xml:"groups>group"`
	Images          []*Image          `xml:"images>image"`
}

type Catalog struct {
	XMLName xml.Name       `xml:"catalog"`
	Items   []*CatalogItem `xml:"items>item"`
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

func getSiteMap() (error) {
	return lib.DownloadAndSave(SipeMapUrl, SiteMapPath, "")
}

func getPage(index int, url *SiteMapUrl, c chan struct{}, e chan error) {
	fileName := PagesDataPath + "page" + strconv.Itoa(index) + ".html"
	log.Println("** " + strconv.Itoa(index) + ".html :" + url.Url)
	if err := lib.DownloadAndSave(url.Url, fileName, ""); err != nil {
		e <- err
	}
	c <- struct{}{}
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

	bar := pb.StartNew(len(siteMap.Urls)).Prefix("Total")
	bar.SetWidth(80)
	bar.ShowSpeed = true
	bar.Start()

	finded := 0
	sem := make(chan struct{}, 10)
	for index, url := range siteMap.Urls {
		bar.Increment()
		if !strings.Contains(url.Url, "/PC/") {
			continue;
		}
		sem <- struct{}{}
		finded++
		bar.Postfix(", find: " + strconv.Itoa(finded))

		fileName := PagesDataPath + "page" + strconv.Itoa(index) + ".html"
		go lib.DownloadAndSaveSem(url.Url, fileName, sem, "windows-1251")
	}

	return nil
}

func parsePage(filename string) (*CatalogItem, error) {

	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		log.Fatal(err)
	}

	category := strings.TrimSpace(doc.Find("[itemprop=\"title\"]").Text())
	if category != "Настольные компьютеры" {
		return nil, nil
	}

	item := new(CatalogItem)
	item.Name = strings.TrimSpace(doc.Find(".title-big[itemprop=\"name\"]").Text())
	item.AttributeGroups = make([]*AttributeGroup, 0)
	doc.Find(".b-product-card-tale table").Each(func(i1 int, s1 *goquery.Selection) {
		group := new(AttributeGroup)
		group.Name = strings.TrimSpace(s1.Find("thead tr th").Text())
		group.Attributes = make([]*Attribute, 0)
		item.AttributeGroups = append(item.AttributeGroups, group)
		s1.Find("tbody tr").Each(func(i2 int, s2 *goquery.Selection) {
			attribute := new(Attribute)
			attribute.Key = strings.TrimSpace(s2.Find("th>span").Text())
			attribute.Value = strings.TrimSpace(s2.Find("td").Text())
			group.Attributes = append(group.Attributes, attribute)
		})
	})

	item.Images = make([]*Image, 0)
	doc.Find("img[itemprop=\"image\"]").Each(func(i1 int, s1 *goquery.Selection) {
		image := new(Image)
		image.Url, _ = s1.Attr("src")
		item.Images = append(item.Images, image)
	})
	return item, nil
}

func parsePages() (error) {

	d, err := ioutil.ReadDir(PagesDataPath)
	if err != nil {
		return err
	}

	catalog := new(Catalog)
	catalog.Items = make([]*CatalogItem, 0)

	bar := pb.StartNew(len(d))
	bar.SetWidth(80)
	bar.ShowSpeed = true

	for _, item := range d {
		bar.Increment()
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

	return saveCatalog(catalog, CatalogPath)
}

func convertPages(directory string) (error) {
	d, err := ioutil.ReadDir(directory)
	if err != nil {
		return err
	}

	for index, item := range d {
		if item.IsDir() {
			continue
		}

		log.Println(strconv.Itoa(index))
		inputF, err := os.Open(directory + item.Name());
		if err != nil {
			log.Println(err)
		}

		utfFile, err := iconv.NewReader(inputF, "windows-1251", "utf-8")
		if err != nil {
			return err
		}
		outputF, err := os.Create(PagesDataPath + item.Name())
		if err != nil {
			log.Println(err)
		}

		io.Copy(outputF, utfFile)
		inputF.Close()
		outputF.Close()
	}

	return nil;
}

func Run() (error) {
	//return getSiteMap();
	//return getPages()
	//convertPages()
	return parsePages()
}
