package middleware

import "testing"

func TestEmailRuleIsStrict(t *testing.T) {
	type req struct {
		Email string `validate:"required,email"`
	}
	for email, ok := range map[string]bool{
		"budi@gmail.com": true, "a.b+c@mail.co.id": true,
		"budi@gmail": false, "budi@": false, "@gmail.com": false, "budi gmail.com": false,
		"budi@@gmail.com": false, "budi@gmail.c": false, "budi@.com": false, "budi@gmail..com": false,
	} {
		err := ValidateRequest(&req{Email: email})
		if (err == nil) != ok {
			t.Errorf("%q: valid=%v, want %v (%v)", email, err == nil, ok, err)
		}
	}
}
