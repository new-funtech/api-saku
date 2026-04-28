package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/internal/config"
	"github.com/ganiramadhan/ganipedia/backend/internal/model"
	repository "github.com/ganiramadhan/ganipedia/backend/internal/repository"
	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/google/uuid"
)

const (
	cacheTTL       = 5 * time.Minute
	cacheKeyPrefix = "product:"
	listKeyPrefix  = "products:list:"
)

type ProductService interface {
	GetAllProducts(ctx context.Context, page, limit int) ([]model.ProductResponse, *model.PaginationMeta, error)
	GetProductByID(ctx context.Context, id uuid.UUID) (*model.ProductResponse, error)
	CreateProduct(ctx context.Context, req model.CreateProductRequest) (*model.ProductResponse, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, req model.UpdateProductRequest) (*model.ProductResponse, error)
	DeleteProduct(ctx context.Context, id uuid.UUID) error
}

type productServiceImpl struct {
	repo repository.ProductRepository
}

func NewProductService(repo repository.ProductRepository) ProductService {
	return &productServiceImpl{repo: repo}
}

type cachedProductList struct {
	Products []model.Product `json:"products"`
	Total    int64           `json:"total"`
}

func (s *productServiceImpl) GetAllProducts(ctx context.Context, page, limit int) ([]model.ProductResponse, *model.PaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

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
		products, total, err = s.repo.FindAll(ctx, page, limit)
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

func (s *productServiceImpl) GetProductByID(ctx context.Context, id uuid.UUID) (*model.ProductResponse, error) {
	cacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, id.String())

	var product *model.Product
	var cachedProduct model.Product
	if s.getFromCache(ctx, cacheKey, &cachedProduct) {
		product = &cachedProduct
	}

	if product == nil {
		var err error
		product, err = s.repo.FindByID(ctx, id)
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

func (s *productServiceImpl) CreateProduct(ctx context.Context, req model.CreateProductRequest) (*model.ProductResponse, error) {
	product := model.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Image:       req.Image,
	}

	if err := s.repo.Create(ctx, &product); err != nil {
		return nil, err
	}

	s.invalidateListCache(ctx)

	resp := toProductResponse(product)
	if product.Image != "" {
		url, _ := utils.GeneratePresignedURL(ctx, product.Image, 24*time.Hour)
		resp.ImageURL = url
	}
	return &resp, nil
}

func (s *productServiceImpl) UpdateProduct(ctx context.Context, id uuid.UUID, req model.UpdateProductRequest) (*model.ProductResponse, error) {
	product, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Image != "" && req.Image != product.Image && product.Image != "" {
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

	if err = s.repo.Update(ctx, product); err != nil {
		return nil, err
	}

	s.invalidateProductCache(ctx, id)

	resp := toProductResponse(*product)
	if product.Image != "" {
		url, _ := utils.GeneratePresignedURL(ctx, product.Image, 24*time.Hour)
		resp.ImageURL = url
	}
	return &resp, nil
}

func (s *productServiceImpl) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	product, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if product.Image != "" {
		_ = utils.DeleteFileFromS3(ctx, product.Image)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.invalidateProductCache(ctx, id)
	return nil
}

func (s *productServiceImpl) getFromCache(ctx context.Context, key string, dest interface{}) bool {
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

func (s *productServiceImpl) setCache(ctx context.Context, key string, value interface{}, ttl time.Duration) {
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

// invalidateProductCache invalidates cache for a specific product and all list caches
func (s *productServiceImpl) invalidateProductCache(ctx context.Context, id uuid.UUID) {
	if config.RedisClient == nil {
		return
	}

	// Delete specific product cache
	productKey := fmt.Sprintf("%s%s", cacheKeyPrefix, id.String())
	config.RedisClient.Del(ctx, productKey)

	// Delete all list caches
	s.invalidateListCache(ctx)

	log.Printf("Invalidated cache for product %s", id.String())
}

// invalidateListCache invalidates all product list caches
func (s *productServiceImpl) invalidateListCache(ctx context.Context) {
	if config.RedisClient == nil {
		return
	}

	iter := config.RedisClient.Scan(ctx, 0, listKeyPrefix+"*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		config.RedisClient.Del(ctx, keys...)
		log.Printf("Invalidated %d product list cache keys", len(keys))
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
