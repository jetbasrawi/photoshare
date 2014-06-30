package routes

import (
	"github.com/danjac/photoshare/api/email"
	"github.com/danjac/photoshare/api/models"
	"github.com/zenazn/goji"
	"regexp"
)

var (
	mailer       = email.NewMailer()
	photoMgr     = models.NewPhotoManager()
	userMgr      = models.NewUserManager()
	ownerUrl     = regexp.MustCompile(`/api/photos/owner/(?P<ownerID>\d+)$`)
	photoUrl     = regexp.MustCompile(`/api/photos/(?P<id>\d+)$`)
	titleUrl     = regexp.MustCompile(`/api/photos/(?P<id>\d+)/title$`)
	tagsUrl      = regexp.MustCompile(`/api/photos/(?P<id>\d+)/tags$`)
	downvoteUrl  = regexp.MustCompile(`/api/photos/(?P<id>\d+)/downvote$`)
	upvoteUrl    = regexp.MustCompile(`/api/photos/(?P<id>\d+)/upvote$`)
	ownerFeedUrl = regexp.MustCompile(`/feeds/owner/(?P<ownerID>\d+)$`)
)

func init() {

	goji.Get("/api/photos/", getPhotos)
	goji.Post("/api/photos/", upload)
	goji.Get("/api/photos/search", searchPhotos)
	goji.Get(ownerUrl, photosByOwnerID)
	goji.Get(photoUrl, photoDetail)
	goji.Delete(photoUrl, deletePhoto)

	goji.Patch(titleUrl, editPhotoTitle)
	goji.Patch(tagsUrl, editPhotoTags)
	goji.Patch(downvoteUrl, voteDown)
	goji.Patch(upvoteUrl, voteUp)

	goji.Get("/api/auth/", authenticate)
	goji.Post("/api/auth/", login)
	goji.Delete("/api/auth/", logout)
	goji.Post("/api/auth/signup", signup)
	goji.Put("/api/auth/recoverpass", recoverPassword)
	goji.Put("/api/auth/changepass", changePassword)

	goji.Get("/api/tags/", getTags)

	goji.Get("/feeds/", latestFeed)
	goji.Get("/feeds/popular/", popularFeed)
	goji.Get(ownerFeedUrl, ownerFeed)

	goji.Handle("/api/messages/*", messageHandler)
}
