package route

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/eggs-gd/perceplib/api"
	"github.com/labstack/echo/v4"
)

// itemFile: one file of the item's group, for the info panel (sidecars too)
type itemFile struct {
	Name string `json:"name"`
	Role string `json:"role"` // original, edit, still, motion, frames, meta; "" not media
	Mime string `json:"mime"`
	Size int64  `json:"size"`
	W    int    `json:"w,omitempty"`
	H    int    `json:"h,omitempty"`
	URL  string `json:"url"` // relative to the API: a download
}

func (r *routes) registerAssets(segment string, e *echo.Echo) {
	userGroup := e.Group(segment)
	userGroup.GET("/:item", r.getFile)             // the default preview
	userGroup.GET("/:item/:file", r.getAssetFile)  // any file of the asset (the asset contract)
	userGroup.GET("/:item/r/:name", r.getRendered) // a rendition of ours (the asset contract's stills)
	e.GET("/items/:guid/files", r.getItemFiles)
}

// r.getItemFiles: every file of the item's group — the original, its edits,
// derivatives, sidecars, frames — each with a URL to download it
func (r *routes) getItemFiles(c echo.Context) error {
	guid := api.GUID(c.Param("guid"))
	files, err := r.db.GetLinkedFiles(guid)
	if err != nil {
		return err
	}
	out := []itemFile{}
	for _, f := range files {
		out = append(out, itemFile{Name: f.Name, Role: f.Role, Mime: f.MimeType, Size: f.Size,
			W: f.Width, H: f.Height, URL: fmt.Sprintf("/assets/%s/%d", guid, f.ID)})
	}
	return c.JSON(http.StatusOK, out)
}

func (r *routes) getFile(c echo.Context) error {
	guid := api.GUID(c.Param("item"))

	item, err := r.db.GetItemByGUID(guid)
	if err != nil {
		return err
	}

	// The cheap preview until our own previews exist; items from before the cheap
	// stage: the original
	if item.PreviewPath != "" {
		return c.File(item.PreviewPath)
	}
	return c.File(item.Path)
}

// r.getAssetFile serves a file of the asset by its id (or the extracted embedded
// preview) — only a file linked to this asset, never an arbitrary path
func (r *routes) getAssetFile(c echo.Context) error {
	guid, name := api.GUID(c.Param("item")), c.Param("file")

	if name == embeddedName {
		item, err := r.db.GetItemByGUID(guid)
		if err != nil || item.PreviewPath == "" {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		return c.File(item.PreviewPath)
	}

	id, err := strconv.ParseUint(name, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	f, err := r.db.GetFileByID(uint(id))
	if err != nil || f.LinkedTo != guid {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	// ?download=1: saved under its own name (the download attribute does not work
	// across origins — the API is on another port)
	if c.QueryParam("download") == "1" {
		return c.Attachment(f.Path, f.Name)
	}
	return c.File(f.Path)
}

// r.getRendered serves one of the item's renditions by its name (<size>.<format>) —
// only one the database lists for it, never an arbitrary path
func (r *routes) getRendered(c echo.Context) error {
	guid, name := api.GUID(c.Param("item")), c.Param("name")
	renditions, err := r.db.Renditions(guid)
	if err != nil {
		return err
	}
	for _, rendition := range renditions {
		if renditionName(rendition) == name {
			return c.File(filepath.Join(r.cache, rendition.Path))
		}
	}
	return echo.NewHTTPError(http.StatusNotFound)
}
