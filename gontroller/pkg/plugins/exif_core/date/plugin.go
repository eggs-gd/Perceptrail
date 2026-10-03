package date

import (
	"perceptrail/gontroller/pkg/plugins/exif_core"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	"github.com/eggs-gd/perceplib/exif"
	l "github.com/eggs-gd/perceplib/logger"
)

type datePerceptor struct{}

func (p *datePerceptor) Name() string                       { return "exif_date" }
func (p *datePerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *datePerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *datePerceptor) Decorator(logger *l.Logger) chain.Decorator[exif_core.RawItemRW, exif_core.RawItemRW] {
	return &datesExtractor{logger}
}

// ExifTags: what resolveDate reads (the dates, their sub-seconds and offsets, the
// GPS time, the coordinates for the zone, the MIME type)
func (p *datePerceptor) ExifTags() []string {
	return append([]string{
		"MIMEType", "CreationDate", "CreateDate",
		"DateTimeOriginal", "SubSecTimeOriginal", "OffsetTimeOriginal",
		"SubSecTimeDigitized", "OffsetTimeDigitized",
		"ModifyDate", "SubSecTime", "OffsetTime",
		"GPSDateTime", "GPSDateStamp", "GPSTimeStamp", "FileModifyDate",
	}, exif.CoordinateTags...)
}

var Perceptor api.Perceptor = &datePerceptor{}
