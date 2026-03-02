package product

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	productRepo "github.com/ganiramadhan/ganipedia/backend/internal/repository/product"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
)

const (
	cacheTTL       = 5 * time.Minute
	cacheKeyPrefix = "product:"
	listKeyPrefix  = "products:list:"
)

type Service interface {
	GetAllProducts(page, limit int) ([]model.ProductResponse, *model.PaginationMeta, error)
	GetProductByID(id uuid.UUID) (*model.ProductResponse, error)
	CreateProduct(req model.CreateProductRequest) (*model.ProductResponse, error)
	UpdateProduct(id uuid.UUID, req model.UpdateProductRequest) (*model.ProductResponse, error)
	DeleteProduct(id uuid.UUID) error
}

type service struct {
	repo productRepo.Repository
}

func NewService(repo productRepo.Repository) Service {
	return &service{repo: repo}
}

type cachedProductList struct {
	Products []model.Product `json:"products"`
	Total    int64           `json:"total"`
}

func (s *service) GetAllProducts(page, limit int) ([]model.ProductResponse, *model.PaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	ctx := context.Background()
	cacheKey := fmt.Sprintf("%spage:%d:limit:%d", listKeyPrefix, page, limit)

	var products []model.Product
	var total int64

	var cachedData cachedProductList
	if s.getFromCache(ctx, cacheKey, &cachedData) {
		products = cachedData.Products
		total = cachedData.Total
	}

	if products == nil {
		var err error
		products, total, err = s.repo.FindAll(page, limit)
		if err != nil {
			return nil, nil, err
		}

		// Store in cache
		s.setCache(ctx, cacheKey, cachedProductList{Products: products, Total: total}, cacheTTL)
	}

	responses := make([]model.ProductResponse, 0, len(products))
	for _, p := range products {
		resp := toProductResponse(p)
		if p.Image != "" {
			url, _ := utils.GeneratePresignedURL(ctx, p.Image, 24*time.Hour)
			resp.ImageURL = url
		}
		responses = append(responses, resp)
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	meta := &model.PaginationMeta{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	return responses, meta, nil
}

func (s *service) GetProductByID(id uuid.UUID) (*model.ProductResponse, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, id.String())

	var product *model.Product
	var cachedProduct model.Product
	if s.getFromCache(ctx, cacheKey, &cachedProduct) {
		product = &cachedProduct
	}

	if product == nil {
		var err error
		product, err = s.repo.FindByID(id)
		if err != nil {
			return nil, err
		}

		s.setCache(ctx, cacheKey, product, cacheTTL)
	}

	resp := toProductResponse(*product)
	if product.Image != "" {
		url, _ := utils.GeneratePresignedURL(ctx, product.Image, 24*time.Hour)
		resp.ImageURL = url
	}
	return &resp, nil
}

func (s *service) CreateProduct(req model.CreateProductRequest) (*model.ProductResponse, error) {
	product := model.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Image:       req.Image,
	}

	if err := s.repo.Create(&product); err != nil {
		return nil, err
	}

	s.invalidateCache()

	resp := toProductResponse(product)
	if product.Image != "" {
		ctx := context.Background()
		url, _ := utils.GeneratePresignedURL(ctx, product.Image, 24*time.Hour)
		resp.ImageURL = url
	}
	return &resp, nil
}

func (s *service) UpdateProduct(id uuid.UUID, req model.UpdateProductRequest) (*model.ProductResponse, error) {
	product, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.Image != "" && req.Image != product.Image && product.Image != "" {
		ctx := context.Background()
		_ = utils.DeleteFileFromS3(ctx, product.Image)
	}

	if req.Name != "" {
		product.Name = req.Name
	}
	if req.Description != "" {
		product.Description = req.Description
	}
	if req.Price != nil {
		product.Price = *req.Price
	}
	if req.Stock != nil {
		product.Stock = *req.Stock
	}
	if req.Image != "" {
		product.Image = req.Image
	}

	if err = s.repo.Update(product); err != nil {
		return nil, err
	}

	s.invalidateCache()

	resp := toProductResponse(*product)
	if product.Image != "" {
		ctx := context.Background()
		url, _ := utils.GeneratePresignedURL(ctx, product.Image, 24*time.Hour)
		resp.ImageURL = url
	}
	return &resp, nil
}

func (s *service) DeleteProduct(id uuid.UUID) error {
	product, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	if product.Image != "" {
		ctx := context.Background()
		_ = utils.DeleteFileFromS3(ctx, product.Image)
	}

	if err := s.repo.Delete(id); err != nil {
		return err
	}

	s.invalidateCache()
	return nil
}

func (s *service) getFromCache(ctx context.Context, key string, dest interface{}) bool {
	if config.RedisClient == nil {
		return false
	}
	cached, err := config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(cached), dest); err != nil {
		return false
	}
	log.Printf("Cache hit for %s", key)
	return true
}

func (s *service) setCache(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	if config.RedisClient == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	if err := config.RedisClient.Set(ctx, key, data, ttl).Err(); err != nil {
		log.Printf("Warning: failed to cache %s: %v", key, err)
	}
}

func (s *service) invalidateCache() {
	if config.RedisClient == nil {
		return
	}
	ctx := context.Background()

	iter := config.RedisClient.Scan(ctx, 0, "product*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		config.RedisClient.Del(ctx, keys...)
		log.Printf("Invalidated %d product cache keys", len(keys))
	}
}

func toProductResponse(p model.Product) model.ProductResponse {
	return model.ProductResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       p.Stock,
		Image:       p.Image,
	}
}
