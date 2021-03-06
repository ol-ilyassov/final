package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/ol-ilyassov/final/article_hub/articlepb"
	"github.com/ol-ilyassov/final/article_hub/authpb"
	"github.com/ol-ilyassov/final/article_hub/notifypb"
	"github.com/ol-ilyassov/final/article_hub/pkg/forms"
	"github.com/ol-ilyassov/final/article_hub/pkg/models"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

func StringToTime(value string) time.Time {
	result, _ := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", value)
	return result
}

func (app *application) home(w http.ResponseWriter, r *http.Request) {

	req := &articlepb.GetArticlesRequest{}
	stream, err := app.articles.GetArticles(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling GetArticles RPC: %v", err)
	}
	defer stream.CloseSend()

	var articles []*models.Article

LOOP:
	for {
		res, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break LOOP
			}
			log.Fatalf("Error while receiving from GetArticles RPC: %v", err)
		}
		log.Printf("Response from GetArticles RPC, ArticleID: %v \n", res.GetArticle().GetId())

		reqUser := &authpb.GetUserRequest{
			Id: res.GetArticle().GetAuthorid(),
		}
		resUser, err := app.auth.GetUser(context.Background(), reqUser)
		if err != nil {
			log.Fatalf("Error while calling GetUser RPC: %v", err)
		}

		tempArticle := &models.Article{
			ID:         int(res.GetArticle().GetId()),
			AuthorID:   int(res.GetArticle().GetAuthorid()),
			AuthorName: resUser.GetUser().GetName(),
			Title:      res.GetArticle().GetTitle(),
			Content:    res.GetArticle().GetContent(),
			Created:    StringToTime(res.GetArticle().GetCreated()),
			Expires:    StringToTime(res.GetArticle().GetExpires()),
		}
		articles = append(articles, tempArticle)
	}

	// Web Design
	app.render(w, r, "home.page.tmpl", &templateData{
		Articles: articles,
	})
}

func (app *application) showArticle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	req := &articlepb.GetArticleRequest{
		Id: int32(id),
	}
	res, err := app.articles.GetArticle(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling GetArticle RPC: %v", err)
	}
	log.Printf("Response from GetArticle RPC: %s, ArticleID: %v", res.GetResult(), res.GetArticle().GetId())

	reqUser := &authpb.GetUserRequest{
		Id: res.GetArticle().GetAuthorid(),
	}

	resUser, err := app.auth.GetUser(context.Background(), reqUser)
	if err != nil {
		log.Fatalf("Error while calling GetUser RPC: %v", err)
	}

	article := &models.Article{
		ID:         int(res.GetArticle().GetId()),
		AuthorID:   int(res.GetArticle().GetAuthorid()),
		AuthorName: resUser.GetUser().GetName(),
		Title:      res.GetArticle().GetTitle(),
		Content:    res.GetArticle().GetContent(),
		Created:    StringToTime(res.GetArticle().GetCreated()),
		Expires:    StringToTime(res.GetArticle().GetExpires()),
	}

	// Web Design
	app.render(w, r, "show.page.tmpl", &templateData{
		Article: article,
		UserID:  app.session.GetInt(r, "authenticatedUserID"),
	})
}

func (app *application) createArticleForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "create.page.tmpl", &templateData{
		Form: forms.New(nil),
	})
}

func (app *application) createArticle(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := forms.New(r.PostForm)
	form.Required("title", "content", "expires")
	form.MaxLength("title", 100)
	form.PermittedValues("expires", "365", "7", "1")
	if !form.Valid() {
		app.render(w, r, "create.page.tmpl", &templateData{Form: form})
		return
	}

	req := &articlepb.InsertArticleRequest{
		Article: &articlepb.Article{
			Title:    form.Get("title"),
			Content:  form.Get("content"),
			Expires:  form.Get("expires"),
			Authorid: int32(app.session.GetInt(r, "authenticatedUserID")),
		},
	}
	res, err := app.articles.InsertArticle(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling InsertArticle RPC: %v", err)
	} else {
		reqUser := &authpb.GetUserRequest{
			Id: int32(app.session.GetInt(r, "authenticatedUserID")),
		}
		resUser, err := app.auth.GetUser(context.Background(), reqUser)
		if err != nil {
			log.Fatalf("Error while calling GetUser RPC: %v", err)
		}
		reqNotify := &notifypb.ArticleCreationRequest{
			Address: resUser.GetUser().GetEmail(),
			Title:   form.Get("title"),
			Time:    time.Now().Format("02 Jan 2006 at 15:04"),
		}
		_, err = app.notifier.ArticleCreationNotify(context.Background(), reqNotify)
		if err != nil {
			log.Fatalf("Error while calling ArticleCreationNotify RPC: %v", err)
		}
	}
	log.Printf("Response from InsertArticle RPC: %v", res.GetResult())

	app.session.Put(r, "flash", "Article successfully created!")
	http.Redirect(w, r, fmt.Sprintf("/article/%d", res.GetId()), http.StatusSeeOther)
}

func (app *application) deleteArticle(w http.ResponseWriter, r *http.Request) {

	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	req := &articlepb.DeleteArticleRequest{
		Id: int32(id),
	}
	res, err := app.articles.DeleteArticle(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling DeleteArticle RPC: %v", err)
	}
	log.Printf("Response from DeleteArticle RPC: %v", res.GetResult())

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) signupUserForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "signup.page.tmpl", &templateData{
		Form: forms.New(nil),
	})
}

func (app *application) signupUser(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}
	form := forms.New(r.PostForm)
	form.Required("name", "email", "password")
	form.MaxLength("name", 255)
	form.MaxLength("email", 255)
	form.MatchesPattern("email", forms.EmailRX)
	form.MinLength("password", 10)
	if !form.Valid() {
		app.render(w, r, "signup.page.tmpl", &templateData{Form: form})
		return
	}

	req := &authpb.CreateUserRequest{
		User: &authpb.User{
			Name:     form.Get("name"),
			Email:    form.Get("email"),
			Password: form.Get("password"),
		},
	}
	res, _ := app.auth.CreateUser(context.Background(), req)
	if !res.GetStatus() {
		if res.GetResult() == models.ErrDuplicateEmail.Error() {
			form.Errors.Add("email", "Address is already in use")
			app.render(w, r, "signup.page.tmpl", &templateData{Form: form})
		} else {
			app.serverError(w, errors.New(res.GetResult()))
		}
		return
	}

	reqNotify := &notifypb.UserCreationRequest{
		Address: form.Get("email"),
		Time:    time.Now().Format("02 Jan 2006 at 15:04"),
	}
	_, err = app.notifier.UserCreationNotify(context.Background(), reqNotify)
	if err != nil {
		log.Fatalf("Error while calling SendNotification RPC: %v", err)
	}

	app.session.Put(r, "flash", "Your signup was successful. Please log in.")

	http.Redirect(w, r, "/user/login", http.StatusSeeOther)
}

func (app *application) loginUserForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "login.page.tmpl", &templateData{
		Form: forms.New(nil),
	})
}

func (app *application) loginUser(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := forms.New(r.PostForm)
	form.Required("email", "password")
	if !form.Valid() {
		form.Errors.Add("generic", "Email and Password are required")
		app.render(w, r, "login.page.tmpl", &templateData{Form: form})
		return
	}

	req := &authpb.AuthUserRequest{
		User: &authpb.User{
			Email:    form.Get("email"),
			Password: form.Get("password"),
		},
	}

	res, _ := app.auth.AuthUser(context.Background(), req)

	if !res.GetStatus() {
		if res.GetResult() == models.ErrInvalidCredentials.Error() {
			form.Errors.Add("generic", "Email or Password is incorrect")
			app.render(w, r, "login.page.tmpl", &templateData{Form: form})
		} else if res.GetResult() == models.ErrNoRecord.Error() {
			form.Errors.Add("generic", "No user with given Email")
			app.render(w, r, "login.page.tmpl", &templateData{Form: form})
		} else {
			app.serverError(w, errors.New(res.GetResult()))
		}
		return
	}

	app.session.Put(r, "authenticatedUserID", int(res.GetId()))

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) logoutUser(w http.ResponseWriter, r *http.Request) {
	app.session.Remove(r, "authenticatedUserID")

	app.session.Put(r, "flash", "You've been logged out successfully!")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) searchForm(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "search.page.tmpl", &templateData{
		Form: forms.New(nil),
	})
}

func (app *application) search(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := forms.New(r.PostForm)
	form.Required("title")
	if !form.Valid() {
		app.render(w, r, "search.page.tmpl", &templateData{Form: form})
		return
	}
	req := &articlepb.SearchArticlesRequest{
		Title: form.Get("title"),
	}
	stream, err := app.articles.SearchArticles(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling SearchArticles RPC: %v", err)
	}
	defer stream.CloseSend()
	var articles []*models.Article

LOOP:
	for {
		res, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break LOOP
			}
			log.Fatalf("Error while receiving from SearchArticles RPC: %v", err)
		}
		log.Printf("Response from SearchArticles RPC, ArticleTitle: %v \n", res.GetArticle().GetTitle())

		reqUser := &authpb.GetUserRequest{
			Id: res.GetArticle().GetAuthorid(),
		}

		resUser, err := app.auth.GetUser(context.Background(), reqUser)
		if err != nil {
			log.Fatalf("Error while calling GetUser RPC: %v", err)
		}

		tempArticle := &models.Article{
			ID:         int(res.GetArticle().GetId()),
			AuthorID:   int(res.GetArticle().GetAuthorid()),
			AuthorName: resUser.GetUser().GetName(),
			Title:      res.GetArticle().GetTitle(),
			Content:    res.GetArticle().GetContent(),
			Created:    StringToTime(res.GetArticle().GetCreated()),
			Expires:    StringToTime(res.GetArticle().GetExpires()),
		}
		articles = append(articles, tempArticle)
	}
	app.render(w, r, "search.page.tmpl", &templateData{
		Articles: articles,
	})
}
func (app *application) editArticleForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}
	req2 := &articlepb.GetArticleRequest{
		Id: int32(id),
	}
	res2, err := app.articles.GetArticle(context.Background(), req2)
	if err != nil {
		log.Fatalf("Error while calling GetArticle RPC: %v", err)
	}
	log.Printf("Response from GetArticle RPC: %s, ArticleID: %v", res2.GetResult(), res2.GetArticle().GetId())

	r.PostFormValue("title")
	r.PostFormValue("content")
	form := forms.New(r.PostForm)
	article := &models.Article{
		ID:      id,
		Title:   res2.GetArticle().GetTitle(),
		Content: res2.GetArticle().GetContent(),
	}

	form.Set("title", article.Title)
	form.Set("content", article.Content)

	app.render(w, r, "edit.page.tmpl", &templateData{
		Form: form, Article: article,
	})
}

func (app *application) editArticle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get(":id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form := forms.New(r.PostForm)
	form.Required("title", "content")
	form.MaxLength("title", 100)
	if !form.Valid() {
		app.render(w, r, "edit.page.tmpl", &templateData{Form: form, Article: &models.Article{ID: id}})
		return
	}

	req := &articlepb.EditArticleRequest{
		Article: &articlepb.Article{
			Title:   form.Get("title"),
			Content: form.Get("content"),
			Id:      int32(id),
		},
	}
	res, err := app.articles.EditArticle(context.Background(), req)
	if err != nil {
		log.Fatalf("Error while calling EditArticle RPC: %v", err)
	}
	log.Printf("Response from EditArticle RPC: %v", res.GetResult())
	http.Redirect(w, r, "/", http.StatusSeeOther)

}
