package main

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
	"strconv"
	"time"
)

var m = &ArticleModel{}

type ArticleModel struct {
	DB *pgxpool.Pool
}

func (m *ArticleModel) Insert(title, content, expires string) (int, error) {

	stmt := "INSERT INTO snippets (title, content, created, expires) VALUES ($1, $2, $3, $4) RETURNING id"

	var id uint64
	days, err := strconv.Atoi(expires)
	if err != nil {
		return 0, err
	}
	row := m.DB.QueryRow(context.Background(), stmt, title, content, time.Now(), time.Now().AddDate(0, 0, days))
	err = row.Scan(&id)
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (m *ArticleModel) Get(id int) (*Article, error) {

	stmt := "SELECT id, title, content, created, expires FROM snippets WHERE expires > $1 AND id = $2"

	row := m.DB.QueryRow(context.Background(), stmt, time.Now(), id)
	s := &Article{}

	err := row.Scan(&s.ID, &s.Title, &s.Content, &s.Created, &s.Expires)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNoRecord
		} else {
			return nil, err
		}
	}
	return s, nil
}

func (m *ArticleModel) Latest() ([]*Article, error) {

	stmt := "SELECT id, title, content, created, expires FROM snippets WHERE expires > $1 ORDER BY created DESC LIMIT 10"

	rows, err := m.DB.Query(context.Background(), stmt, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snippets []*Article

	for rows.Next() {
		s := &Article{}
		err = rows.Scan(&s.ID, &s.Title, &s.Content, &s.Created, &s.Expires)
		if err != nil {
			return nil, err
		}
		snippets = append(snippets, s)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return snippets, nil
}
