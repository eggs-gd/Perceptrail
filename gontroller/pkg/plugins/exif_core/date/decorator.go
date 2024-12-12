package date

import (
	"perceptrail/api"
	"perceptrail/chain"
	"perceptrail/gontroller/pkg/plugins/exif_core"
	"time"

	l "perceptrail/logger"
)

type datesExtractor struct {
	logger *l.Logger
}

func (cd *datesExtractor) Decorate(in exif_core.RawItemRW) (exif_core.RawItemRW, error) {
	date := extractBestDate(in)
	if !date.IsZero() && in.GetDate() != date {
		in.SetDate(date)
	}
	return in, nil
}

func (cd *datesExtractor) Stop() {}

func NewDatesProcessor(chin <-chan exif_core.RawItemRW, chout chan<- exif_core.RawItemRW, logger *l.Logger) chain.Processor {
	return chain.NewDecorator(chin, chout, &datesExtractor{logger})
}

// DateFormat represents supported date formats
type DateFormat struct {
	tag    string
	layout string
}

var (
	// Primary date sources - most accurate
	primaryDateFormats = []DateFormat{
		{"DateTimeOriginal", "2006:01:02 15:04:05"},
		{"CreateDate", "2006:01:02 15:04:05"},
	}

	// GPS date source - separate handling due to split date/time format
	gpsDateFormats = []DateFormat{
		{"GPSDateStamp", "2006:01:02"},
		{"GPSTimeStamp", "15:04:05"},
	}

	// Fallback date sources - less accurate
	fallbackDateFormats = []DateFormat{
		{"ModifyDate", "2006:01:02 15:04:05"},
		{"FileModifyDate", "2006:01:02 15:04:05"},
		{"FileCreateDate", "2006:01:02 15:04:05"},
	}
)

func extractBestDate(exifData api.ExifProvider) time.Time {
	extractors := []func(api.ExifProvider) time.Time{
		extractGPSDateTime,  // Try GPS date first (most accurate when available)
		extractPrimaryDate,  // Then try primary sources
		extractFallbackDate, // Finally try fallback sources
	}

	for _, extractor := range extractors {
		if date := extractor(exifData); !date.IsZero() {
			return date
		}
	}

	return time.Time{}
}

func extractPrimaryDate(exifData api.ExifProvider) time.Time {
	return extractDateFromFormats(exifData, primaryDateFormats)
}

func extractFallbackDate(exifData api.ExifProvider) time.Time {
	return extractDateFromFormats(exifData, fallbackDateFormats)
}

func extractDateFromFormats(exifData api.ExifProvider, formats []DateFormat) time.Time {
	for _, df := range formats {
		if date := parseDate(exifData, df); !date.IsZero() {
			return date
		}
	}
	return time.Time{}
}

func parseDate(exifData api.ExifProvider, df DateFormat) time.Time {
	dateStr := exifData.GetExif(df.tag)
	if dateStr == "" {
		return time.Time{}
	}

	date, err := time.Parse(df.layout, dateStr)
	if err != nil {
		return time.Time{}
	}

	return date
}

func extractGPSDateTime(exifData api.ExifProvider) time.Time {
	dateStr := exifData.GetExif(gpsDateFormats[0].tag)
	timeStr := exifData.GetExif(gpsDateFormats[1].tag)

	if dateStr == "" || timeStr == "" {
		return time.Time{}
	}

	// Combine date and time strings
	dateTimeStr := dateStr + " " + timeStr
	dateTime, err := time.Parse("2006:01:02 15:04:05", dateTimeStr)
	if err != nil {
		return time.Time{}
	}

	return dateTime
}
