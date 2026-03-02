package routes

import (
	authHandler "github.com/ganiramadhan/ganipedia/backend/internal/handlers/auth"
	productHandler "github.com/ganiramadhan/ganipedia/backend/internal/handlers/products"
	userHandler "github.com/ganiramadhan/ganipedia/backend/internal/handlers/users"
	"github.com/ganiramadhan/ganipedia/backend/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(
	app *fiber.App,
	authHdl *authHandler.Handler,
	productHdl *productHandler.ProductHandler,
	userHdl *userHandler.UserHandler,
) {
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Auth routes (public)
	auth := v1.Group("/auth")
	auth.Post("/login", authHdl.Login)
	auth.Post("/register", authHdl.Register)

	// Product routes
	productRoutes := v1.Group("/products")
	productRoutes.Get("/", productHdl.GetAllProducts)
	productRoutes.Get("/:id", productHdl.GetProductByID)
	productRoutes.Post("/", middleware.AuthRequired(), productHdl.CreateProduct)
	productRoutes.Post("/upload", middleware.AuthRequired(), productHdl.UploadProductImage)
	productRoutes.Put("/:id", middleware.AuthRequired(), productHdl.UpdateProduct)
	productRoutes.Delete("/:id", middleware.AuthRequired(), productHdl.DeleteProduct)

	// User routes (all protected)
	userRoutes := v1.Group("/users", middleware.AuthRequired())
	userRoutes.Get("/", userHdl.GetAllUsers)
	userRoutes.Get("/:id", userHdl.GetUserByID)
	userRoutes.Post("/", userHdl.CreateUser)
	userRoutes.Put("/:id", userHdl.UpdateUser)
	userRoutes.Delete("/:id", userHdl.DeleteUser)
}
