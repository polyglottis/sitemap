package sitemap

import (
	"fmt"
)

// Buffer is a sitemap buffer.
//
// When the current sitemap is full, it is offloaded to disk, and a new empty sitemap is created.
type Buffer struct {
	sitemap   *Sitemap
	count     int // number of sitemaps
	domain    string
	cachePath string
	Locations []string // Relative path of serialized sitemaps.
}

// NewBuffer creates a new buffer for sitemaps on the given domain. The path variable is the location for serialization on Flush().
func NewBuffer(domain, path string) *Buffer {
	return &Buffer{
		domain:    domain,
		cachePath: path,
	}
}

const sitemap_pattern = "sitemap_%d.xml"

// Flush writes the content of the buffer to a sitemap file and adds the file to the list of locations.
// This occurs only if the buffer is non-empty. Calling Flush on an empty buffer is a no-op.
func (b *Buffer) Flush() error {
	if !b.sitemap.IsEmpty() {
		b.count++
		location := fmt.Sprintf(sitemap_pattern, b.count)
		err := b.sitemap.WriteToFile(b.cachePath + location)
		if err != nil {
			return err
		}
		b.Locations = append(b.Locations, location)
	}
	b.sitemap = nil
	return nil
}

// AddEntry adds an entry to the buffer.
// If the sitemap buffer is full, it calls Flush() before inserting the entry to a new Sitemap.
func (b *Buffer) AddEntry(e *Entry) error {
	if b.sitemap.IsFull() {
		err := b.Flush()
		if err != nil {
			return err
		}
	}
	if b.sitemap == nil {
		b.sitemap = NewSitemap()
	}

	b.sitemap.Entries = append(b.sitemap.Entries, e)
	return nil
}
