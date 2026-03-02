package constants

const (
	SuccessGetProducts   = "Successfully retrieved products"
	SuccessGetProduct    = "Successfully retrieved product"
	SuccessCreateProduct = "Successfully created product"
	SuccessUpdateProduct = "Successfully updated product"
	SuccessDeleteProduct = "Successfully deleted product"
	SuccessUploadImage   = "Image uploaded successfully"

	SuccessGetUsers   = "Successfully retrieved users"
	SuccessGetUser    = "Successfully retrieved user"
	SuccessCreateUser = "Successfully created user"
	SuccessUpdateUser = "Successfully updated user"
	SuccessDeleteUser = "Successfully deleted user"

	SuccessLogin    = "Login successful"
	SuccessRegister = "Registration successful"

	ErrInvalidRequest = "Invalid request body"
	ErrInternalServer = "Internal server error"
	ErrInvalidUUID    = "Invalid UUID format"

	ErrProductNotFound = "Product not found"
	ErrImageRequired   = "Image file is required"
	ErrInvalidFileType = "Invalid file type. Only images are allowed (jpeg, png, gif, webp)"
	ErrFileTooLarge    = "File size too large. Maximum size is 5MB"
	ErrUploadFailed    = "Failed to upload image"
	ErrPreviewFailed   = "Failed to generate preview URL"

	ErrUserNotFound = "User not found"
	ErrEmailExists  = "Email already exists"

	ErrInvalidCredentials = "Invalid email or password"
	ErrUnauthorized       = "Authorization header is required"
	ErrInvalidToken       = "Invalid or expired token"

	MaxUploadSize = 5 * 1024 * 1024 // 5MB
)
