package sitemap

import (
	"encoding/xml"
	"time"
)

// Schema represents an XML schema.
type Schema struct {
	Xmlns             string `xml:"xmlns,attr"`
	XmlnsXsi          string `xml:"xmlns:xsi,attr"`
	XsiSchemaLocation string `xml:"xsi:schemaLocation,attr"`
}

// SitemapSchema is the XML schema used for sitemaps.
var SitemapSchema = &Schema{
	Xmlns:             "http://www.sitemaps.org/schemas/sitemap/0.9",
	XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
	XsiSchemaLocation: "http://www.sitemaps.org/schemas/sitemap/0.9 http://www.sitemaps.org/schemas/sitemap/0.9/sitemap.xsd",
}

// ChangeFrequency is an optional attribute for sitemap entries.
type ChangeFrequency string

const (
	Always  ChangeFrequency = "always"
	Hourly                  = "hourly"
	Daily                   = "daily"
	Weekly                  = "weekly"
	Monthly                 = "monthly"
	Yearly                  = "yearly"
	Never                   = "never"
)

// Sitemap is a sitemap, with xml-encoding attributes.
//
// Call NewSitemap() to get a new sitemap with the correct XML schema, ready to get encoded.
type Sitemap struct {
	XMLName xml.Name `xml:"urlset"`
	*Schema
	Entries []*Entry `xml:"url"`
}

// FileReference is a reference to a file (given by full URL) and the last modification.
type FileReference struct {
	Location         string     `xml:"loc"`
	LastModification *time.Time `xml:"lastmod,omitempty"` // optional
}

// Entry is a sitemap entry (a url block in the XML file).
type Entry struct {
	*FileReference
	ChangeFrequency ChangeFrequency `xml:"changefreq,omitempty"` // optional
	Priority        *float64        `xml:"priority,omitempty"`   // optional
}

// NewSitemap creates an empty sitemap with the schema set as SitemapSchema.
func NewSitemap() *Sitemap {
	return &Sitemap{
		Schema: SitemapSchema,
	}
}

// IsEmpty returns true if the sitemap is empty or nil.
func (s *Sitemap) IsEmpty() bool {
	return s == nil || len(s.Entries) == 0
}

// IsFull retruns true if the sitemap has reached the maximum number of entries allowed.
func (s *Sitemap) IsFull() bool {
	if s == nil {
		return false
	}
	return len(s.Entries) >= 50000
}

// WriteToFile encodes the sitemap in XML format into path.
func (s *Sitemap) WriteToFile(path string) error {
	return writeToFileXML(s, path)
}
