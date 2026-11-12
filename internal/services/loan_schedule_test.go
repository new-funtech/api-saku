package services

import (
	"math"
	"testing"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
)

// The installment schedule must add up to exactly principal + interest, even
// when the monthly amounts do not divide evenly.
func TestApplyDisbursementScheduleAddsUp(t *testing.T) {
	s := &loanServiceImpl{}
	for _, tc := range []struct {
		amount float64
		tenor  int
	}{{1_000_000, 3}, {500_000, 1}, {1_234_567, 3}, {2_000_000, 7}} {
		loan := &model.Loan{LoanAmount: tc.amount, TenorMonths: tc.tenor, InterestRate: 3}
		items := s.applyDisbursement(loan, nil)
		if len(items) != tc.tenor {
			t.Fatalf("tenor %d: got %d installments", tc.tenor, len(items))
		}
		var principal, interest float64
		for _, it := range items {
			principal += it.PrincipalAmount
			interest += it.InterestAmount
		}
		wantInterest := round2(tc.amount * 0.03 * float64(tc.tenor))
		if math.Abs(principal-tc.amount) > 0.001 || math.Abs(interest-wantInterest) > 0.001 {
			t.Errorf("amount %.0f tenor %d: principal %.2f interest %.2f, want %.2f / %.2f",
				tc.amount, tc.tenor, principal, interest, tc.amount, wantInterest)
		}
		if loan.Status != model.LoanStatusActive {
			t.Errorf("status = %s", loan.Status)
		}
	}
}
