package main

import (
	_ "fmt"
	"github.com/gin-gonic/gin"
	"github.com/honeyjonny/sociality/database"
	"github.com/honeyjonny/sociality/middleware"
	"github.com/jinzhu/gorm"
	"net/http"
	_ "time"
)

func main() {

	dbconfig := database.DbConfig{
		Dialect:          "postgres",
		ConnectionString: "user=sadm dbname=social password=ChangeThis sslmode=disable",
	}

	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	router.Static("/public/css/", "./public/css/")

	router.LoadHTMLGlob("./templates/*")

	dbcontext := dbconfig.CreateConnection()

	router.Use(middleware.SetDbContext(dbcontext))

	authorized := router.Group("/", middleware.AuthBySession(dbcontext))

	authorized.GET("/users", func(c *gin.Context) {

		if _, exists := middleware.GetUserFromGinContext(c); exists {

			var usrDtos []middleware.UserDTO

			dbctx := c.MustGet("dbcontext").(*gorm.DB)

			dbctx.
				Table("users").
				Select("user_name as username, created_at as created").
				Scan(&usrDtos)

			c.HTML(http.StatusOK, "users.tmpl", gin.H{
				"title": "Users page",
				"users": usrDtos,
			})
		}
	})

	authorized.GET("/home", func(c *gin.Context) {

		if user, exists := middleware.GetUserFromGinContext(c); exists {

			var posts []middleware.PostDTO

			dbctx := c.MustGet("dbcontext").(*gorm.DB)

			dbctx.
				Table("posts").
				Joins("inner join users on users.id = posts.user_id").
				Where("users.id = ?", user.ID).
				Order("posts.created_at desc").
				Select("posts.created_at as created, posts.text as content").
				Scan(&posts)

			c.HTML(http.StatusOK, "home.tmpl", gin.H{
				"title":    "Home page",
				"username": user.UserName,
				"posts":    posts,
			})
		}
	})

	authorized.POST("/posts", func(c *gin.Context) {
		if user, exists := middleware.GetUserFromGinContext(c); exists {

			var newPost middleware.PostForm

			if c.Bind(&newPost) == nil {

				dbctx := c.MustGet("dbcontext").(*gorm.DB)

				truncatedContent := middleware.TruncatePostContent(newPost.Content)

				dbPost := database.Post{
					UserID: user.ID,
					Text:   truncatedContent,
				}

				dbctx.Create(&dbPost)

				c.Header("Location", "/home")
				c.JSON(http.StatusSeeOther, gin.H{
					"created": dbPost,
				})

			} else {

				c.JSON(http.StatusBadRequest, gin.H{
					"error": "form invalid",
				})
			}
		}
	})

	authorized.GET("/logout", func(c *gin.Context) {
		if user, exists := middleware.GetUserFromGinContext(c); exists {

			dbctx := c.MustGet("dbcontext").(*gorm.DB)

			/*			dbctx.
						Table("sessions").
						Where(&database.Session{UserID: user.ID}).
						Delete(database.Session{})*/

			dbctx.
				Unscoped().
				Table("sessions").
				Where(&database.Session{UserID: user.ID}).
				Delete(database.Session{})

			c.Header("Location", "/")
			c.JSON(http.StatusSeeOther, gin.H{
				"logout": user.UserName,
			})
		}
	})

	router.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.tmpl", gin.H{
			"title": "Register form",
			"body":  "Register, please",
		})

	})

	router.POST("/register", func(c *gin.Context) {
		var newUser middleware.LoginForm

		dbctx := c.MustGet("dbcontext").(*gorm.DB)

		if c.Bind(&newUser) == nil {

			var checkUsr database.User

			notFound :=
				dbctx.
					Where(&database.User{
						UserName: newUser.Username,
					}).
					First(&checkUsr).
					RecordNotFound()

			if !notFound {

				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "user already exists",
				})

			} else {

				dbUser := database.User{
					UserName: newUser.Username,
					Password: newUser.Password,
				}

				dbctx.Create(&dbUser)

				c.Header("Location", "/login")
				c.JSON(http.StatusSeeOther, gin.H{
					"registered": dbUser,
				})

			}

		} else {

			c.JSON(http.StatusBadRequest, gin.H{
				"error": "form invalid",
			})

		}

	})

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.tmpl", gin.H{
			"title": "Login form",
			"body":  "Login, please",
		})

	})

	router.POST("/login", func(c *gin.Context) {
		var form middleware.LoginForm
		var dbUser database.User

		dbctx := c.MustGet("dbcontext").(*gorm.DB)

		if c.Bind(&form) == nil {

			notFound :=
				dbctx.
					Where(&database.User{
						UserName: form.Username,
						Password: form.Password,
					}).
					First(&dbUser).
					RecordNotFound()

			if notFound {

				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "unauthorized",
				})

				return

			} else {

				session, timestamp := middleware.CreateSessionForUser(dbUser)

				dbctx.
					Table("sessions").
					Where(&database.Session{UserID: dbUser.ID}).
					Delete(database.Session{})

				dbctx.
					Unscoped().
					Table("sessions").
					Where(&database.Session{UserID: dbUser.ID}).
					Delete(database.Session{})

				dbctx.
					Create(&session)

				c.SetCookie("_session", session.Cookie, 0, "/", "", false, false)

				c.Header("Location", "/home")
				c.JSON(http.StatusFound, gin.H{
					"logined":   dbUser.UserName,
					"timestamp": timestamp,
				})

				return
			}

		} else {

			c.JSON(http.StatusBadRequest, gin.H{
				"error": "form invalid",
			})

			return
		}
	})

	router.GET("/", func(c *gin.Context) {

		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title": "Default Social Network",
			"body":  "Social network for you and your colleagues!",
		})
	})

	router.Run(":8080")
}