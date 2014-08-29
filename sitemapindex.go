package sitemap

import (
	"encoding/xml"
	"html"
	"os"
)

// SitemapIndex is a sitemap index with xml-encoding attributes.
type SitemapIndex struct {
	XMLName xml.Name `xml:"sitemapindex"`
	Schema
	SitemapRefs []*FileReference `xml:"sitemap"`
}

// SitemapIndexSchema is the XML schema used for sitemap indexes.
var SitemapIndexSchema = Schema{
	Xmlns:             "http://www.sitemaps.org/schemas/sitemap/0.9",
	XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
	XsiSchemaLocation: "http://www.sitemaps.org/schemas/sitemap/0.9 http://www.sitemaps.org/schemas/sitemap/0.9/siteindex.xsd",
}

// NewSitemapIndex creates a sitemap index with the default schema and all sitemap urls given.
func NewSitemapIndex(sitemapUrls []string) *SitemapIndex {
	refs := make([]*FileReference, len(sitemapUrls), len(sitemapUrls))
	for i, loc := range sitemapUrls {
		refs[i] = &FileReference{Location: html.EscapeString(loc)}
	}
	return &SitemapIndex{SitemapRefs: refs, Schema: SitemapIndexSchema}
}

// WriteToFile writes the sitemap index in XML into path.
func (s *SitemapIndex) WriteToFile(path string) error {
	return writeToFileXML(s, path)
}

// writeToFileXML writes the given data into outFileName, using the encoding/xml.
func writeToFileXML(data interface{}, outFileName string) error {
	out, err := os.Create(outFileName)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.Write([]byte(xml.Header))
	if err != nil {
		return err
	}

	encoder := xml.NewEncoder(out)
	encoder.Indent("", "  ")

	return encoder.Encode(data)
}
