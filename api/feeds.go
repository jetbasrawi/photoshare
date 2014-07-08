package api

import (
	"fmt"
	"github.com/gorilla/feeds"
	"github.com/zenazn/goji/web"
	"net/http"
	"strconv"
	"time"
)

func photoFeed(c web.C,
	w http.ResponseWriter,
	r *http.Request,
	title string,
	description string,
	link string,
	photos *PhotoList) {

	baseURL := baseURL(r)

	feed := &feeds.Feed{
		Title:       title,
		Link:        &feeds.Link{Href: baseURL + link},
		Description: description,
		Created:     time.Now(),
	}

	for _, photo := range photos.Items {

		item := &feeds.Item{
			Id:          strconv.FormatInt(photo.ID, 10),
			Title:       photo.Title,
			Link:        &feeds.Link{Href: fmt.Sprintf("%s/#/detail/%d", baseURL, photo.ID)},
			Description: fmt.Sprintf("<img src=\"%s/uploads/thumbnails/%s\">", baseURL, photo.Filename),
			Created:     photo.CreatedAt,
		}
		feed.Add(item)
	}
	atom, err := feed.ToAtom()
	if err != nil {
		handleServerError(w, err)
		return
	}
	writeBody(w, []byte(atom), http.StatusOK, "application/atom+xml")
}

func latestFeed(c web.C, w http.ResponseWriter, r *http.Request) {

	photos, err := photoMgr.All(1, "")

	if err != nil {
		handleServerError(w, err)
		return
	}

	photoFeed(c, w, r, "Latest photos", "Most recent photos", "/latest", photos)
}

func popularFeed(c web.C, w http.ResponseWriter, r *http.Request) {

	photos, err := photoMgr.All(1, "votes")

	if err != nil {
		handleServerError(w, err)
		return
	}

	photoFeed(c, w, r, "Popular photos", "Most upvoted photos", "/popular", photos)
}

func ownerFeed(c web.C, w http.ResponseWriter, r *http.Request) {
	ownerID, err := strconv.ParseInt(c.URLParams["ownerID"], 10, 0)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	owner, exists, err := userMgr.GetActive(ownerID)
	if err != nil {
		handleServerError(w, err)
		return
	}
	if !exists {
		http.NotFound(w, r)
		return
	}

	title := "Feeds for " + owner.Name
	description := "List of feeds for " + owner.Name
	link := fmt.Sprintf("/owner/%d/%s", ownerID, owner.Name)

	photos, err := photoMgr.ByOwnerID(1, ownerID)

	if err != nil {
		handleServerError(w, err)
		return
	}
	photoFeed(c, w, r, title, description, link, photos)
}
