package sensitivity_labels

import "encoding/xml"

type FileLabel struct {
	FilePath  string
	LabelInfo bool
	Labels    []Label
}

type Labels struct {
	XMLName xml.Name `xml:"labelList"`
	Labels  []Label  `xml:"label"`
}

type Label struct {
	XMLName     xml.Name `xml:"label"`
	Id          string   `xml:"id,attr"`
	SiteId      string   `xml:"siteId,attr"`
	Enabled     string   `xml:"enabled,attr"`
	Method      string   `xml:"method,attr"`
	ContentBits string   `xml:"contentBits,attr"`
	Removed     string   `xml:"removed,attr"`
}
