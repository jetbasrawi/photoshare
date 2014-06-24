package models

import (
	"database/sql"
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/danjac/photoshare/api/storage"
	"math"
	"strings"
	"time"
)

const PageSize = 12

type PhotoManager interface {
	Insert(*Photo) error
	Update(*Photo) error
	Delete(*Photo) error
	Get(string) (*Photo, error)
	GetDetail(string, *User) (*PhotoDetail, error)
	GetTagCounts() ([]TagCount, error)
	All(int64, string) (*PhotoList, error)
	ByOwnerID(int64, string) (*PhotoList, error)
	Search(int64, string) (*PhotoList, error)
	UpdateTags(*Photo) error
}

type PhotoList struct {
	Photos      []Photo `json:"photos"`
	Total       int64   `json:"total"`
	CurrentPage int64   `json:"currentPage"`
	NumPages    int64   `json:"numPages"`
}

func NewPhotoList(photos []Photo, total int64, page int64) *PhotoList {
	numPages := int64(math.Ceil(float64(total) / float64(PageSize)))

	return &PhotoList{
		Photos:      photos,
		Total:       total,
		CurrentPage: page,
		NumPages:    numPages,
	}
}

type Tag struct {
	ID   int64  `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
}

type TagCount struct {
	Name      string `db:"name" json:"name"`
	Photo     string `db:"photo" json:"photo"`
	NumPhotos int64  `db:"num_photos" json:"numPhotos"`
}

type Photo struct {
	ID        int64     `db:"id" json:"id"`
	OwnerID   int64     `db:"owner_id" json:"ownerId"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	Title     string    `db:"title" json:"title"`
	Filename  string    `db:"photo" json:"photo"`
	Tags      []string  `db:"-" json:"tags"`
	UpVotes   int64     `db:"up_votes" json:"upVotes"`
	DownVotes int64     `db:"down_votes" json:"downVotes"`
}

var photoCleaner = storage.NewPhotoCleaner()

func (photo *Photo) PreInsert(s gorp.SqlExecutor) error {
	photo.CreatedAt = time.Now()
	return nil
}

func (photo *Photo) PreDelete(s gorp.SqlExecutor) error {
	go photoCleaner.Clean(photo.Filename)
	return nil
}

func (photo *Photo) CanEdit(user *User) bool {
	if user == nil || !user.IsAuthenticated {
		return false
	}
	return user.IsAdmin || photo.OwnerID == user.ID
}

func (photo *Photo) CanDelete(user *User) bool {
	return photo.CanEdit(user)
}

func (photo *Photo) CanVote(user *User) bool {
	if user == nil || !user.IsAuthenticated {
		return false
	}
	if photo.OwnerID == user.ID {
		return false
	}

	return !user.HasVoted(photo.ID)
}

type Permissions struct {
	Edit   bool `db:"-" json:"edit"`
	Delete bool `db:"-" json:"delete"`
	Vote   bool `db:"_" json:"vote"`
}

type PhotoDetail struct {
	Photo       `db:"-"`
	OwnerName   string       `db:"owner_name" json:"ownerName"`
	Permissions *Permissions `db:"-" json:"perms"`
}

type defaultPhotoManager struct{}

var photoMgr = &defaultPhotoManager{}

func NewPhotoManager() PhotoManager {
	return photoMgr
}

func (mgr *defaultPhotoManager) Delete(photo *Photo) error {
	t, err := dbMap.Begin()
	if err != nil {
		return err
	}
	if _, err := dbMap.Delete(photo); err != nil {
		return err
	}

	return t.Commit()
}

func (mgr *defaultPhotoManager) Update(photo *Photo) error {

	_, err := dbMap.Update(photo)
	return err
}

func (mgr *defaultPhotoManager) Insert(photo *Photo) error {
	t, err := dbMap.Begin()
	if err != nil {
		return err
	}
	if err := dbMap.Insert(photo); err != nil {
		return err
	}
	if err := mgr.UpdateTags(photo); err != nil {
		return err
	}
	return t.Commit()
}

func (mgr *defaultPhotoManager) UpdateTags(photo *Photo) error {

	var (
		args    = []string{"$1"}
		params  = []interface{}{interface{}(photo.ID)}
		isEmpty = true
	)
	for i, name := range photo.Tags {
		name = strings.TrimSpace(name)
		if name != "" {
			args = append(args, fmt.Sprintf("$%d", i+2))
			params = append(params, interface{}(strings.ToLower(name)))
			isEmpty = false
		}
	}
	if isEmpty && photo.ID != 0 {
		_, err := dbMap.Exec("DELETE FROM photo_tags WHERE photo_id=$1", photo.ID)
		return err
	}
	_, err := dbMap.Exec(fmt.Sprintf("SELECT add_tags(%s)", strings.Join(args, ",")), params...)
	return err

}

func (mgr *defaultPhotoManager) Get(photoID string) (*Photo, error) {

	photo := &Photo{}
	obj, err := dbMap.Get(photo, photoID)
	if err != nil {
		return photo, err
	}
	if obj == nil {
		return nil, nil
	}
	return obj.(*Photo), nil
}

func (mgr *defaultPhotoManager) GetDetail(photoID string, user *User) (*PhotoDetail, error) {

	photo := &PhotoDetail{}

	q := "SELECT p.*, u.name AS owner_name " +
		"FROM photos p JOIN users u ON u.id = p.owner_id " +
		"WHERE p.id=$1"

	if err := dbMap.SelectOne(photo, q, photoID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var tags []Tag

	if _, err := dbMap.Select(&tags,
		"SELECT t.* FROM tags t JOIN photo_tags pt ON pt.tag_id=t.id "+
			"WHERE pt.photo_id=$1", photo.ID); err != nil {
		return photo, err
	}
	for _, tag := range tags {
		photo.Tags = append(photo.Tags, tag.Name)
	}

	photo.Permissions = &Permissions{
		photo.CanEdit(user),
		photo.CanDelete(user),
		photo.CanVote(user),
	}
	return photo, nil

}

func (mgr *defaultPhotoManager) ByOwnerID(pageNum int64, ownerID string) (*PhotoList, error) {

	var (
		photos []Photo
		err    error
		total  int64
	)

	if total, err = dbMap.SelectInt("SELECT COUNT(id) FROM photos WHERE owner_id=$1", ownerID); err != nil {
		return nil, err
	}

	if _, err = dbMap.Select(&photos,
		"SELECT * FROM photos WHERE owner_id = $1"+
			"ORDER BY (up_votes - down_votes) DESC, created_at DESC LIMIT $2 OFFSET $3",
		ownerID, PageSize, getOffset(pageNum)); err != nil {
		return nil, err
	}
	return NewPhotoList(photos, total, pageNum), nil
}

func (mgr *defaultPhotoManager) Search(pageNum int64, q string) (*PhotoList, error) {

	var (
		clauses []string
		params  []interface{}
		err     error
		photos  []Photo
		total   int64
	)

	if q == "" {
		return nil, nil
	}

	for num, word := range strings.Split(q, " ") {
		word = strings.TrimSpace(word)
		if word == "" || num > 6 {
			break
		}
		word = "%" + word + "%"
		num += 1
		clauses = append(clauses, fmt.Sprintf(
			"SELECT DISTINCT p.* FROM photos p "+
				"INNER JOIN users u ON u.id = p.owner_id  "+
				"LEFT JOIN photo_tags pt ON pt.photo_id = p.id "+
				"LEFT JOIN tags t ON pt.tag_id=t.id "+
				"WHERE UPPER(p.title::text) LIKE UPPER($%d) OR UPPER(u.name::text) LIKE UPPER($%d) OR t.name LIKE $%d",
			num, num, num))

		params = append(params, interface{}(word))
	}

	clausesSql := strings.Join(clauses, " INTERSECT ")

	countSql := fmt.Sprintf("SELECT COUNT(id) FROM (%s) q", clausesSql)
	if total, err = dbMap.SelectInt(countSql, params...); err != nil {
		return nil, err
	}

	numParams := len(params)

	sql := fmt.Sprintf("SELECT * FROM (%s) q ORDER BY (up_votes - down_votes) DESC, created_at DESC LIMIT $%d OFFSET $%d",
		clausesSql, numParams+1, numParams+2)

	params = append(params, interface{}(PageSize))
	params = append(params, interface{}(getOffset(pageNum)))

	if _, err = dbMap.Select(&photos, sql, params...); err != nil {
		return nil, err
	}
	return NewPhotoList(photos, total, pageNum), nil
}

func (mgr *defaultPhotoManager) All(pageNum int64, orderBy string) (*PhotoList, error) {

	var (
		total  int64
		photos []Photo
		err    error
	)
	if orderBy == "votes" {
		orderBy = "(up_votes - down_votes)"
	} else {
		orderBy = "created_at"
	}

	if total, err = dbMap.SelectInt("SELECT COUNT(id) FROM photos"); err != nil {
		return nil, err
	}

	if _, err = dbMap.Select(&photos,
		"SELECT * FROM photos "+
			"ORDER BY "+orderBy+" DESC LIMIT $1 OFFSET $2", PageSize, getOffset(pageNum)); err != nil {
		return nil, err
	}
	return NewPhotoList(photos, total, pageNum), nil
}

func (mgr *defaultPhotoManager) GetTagCounts() ([]TagCount, error) {
	var tags []TagCount
	if _, err := dbMap.Select(&tags, "SELECT name, photo, num_photos FROM tag_counts"); err != nil {
		return tags, err
	}
	return tags, nil
}

func getOffset(pageNum int64) int64 {
	return (pageNum - 1) * PageSize
}
