package assessment

// Expert choices, never trained coefficients or calibrated default probabilities.
// Age is admission-only; existing debt is affordability-only; data quality is a
// separate result. The three groups have no amount/term/rate dependence.
const scoreConfig = `{"model":"expert-cashflow-scorecard-v2","baseline":0,"payment_discipline":{"weight_bps":4000,"dpd_upper":[0,7,29,59,89],"points":[400,320,200,80,0]},"operating_margin":{"weight_bps":3500,"bps_lower":[2000,1500,1000,500,0],"points":[350,280,210,105,35]},"customer_concentration":{"weight_bps":2500,"bps_upper":[2000,3500,5000,7500,10000],"points":[250,200,125,50,0]},"grades":{"A":800,"B":650,"C":500,"D":350,"E":0}}`

func Score(f FinancialSnapshot, c CreditHistory) RiskResult {
	r := RiskResult{Status: "INSUFFICIENT_DATA", ModelVersion: ModelVersion, ConfigChecksum: hash(scoreConfig), Factors: []Factor{}, Issues: []Issue{}}
	if c.HistoricalMaxDPD == nil {
		r.Issues = append(r.Issues, issue("MISSING_VALUE", "credit.historical_max_dpd_12m", "Отсутствует платёжная история"))
	}
	if f.LargestCustomerShareBPS == nil {
		r.Issues = append(r.Issues, issue("MISSING_VALUE", "financials.largest_customer_share_bps", "Укажите долю крупнейшего покупателя в операционных поступлениях"))
	} else if *f.LargestCustomerShareBPS < 0 || *f.LargestCustomerShareBPS > 10000 {
		r.Issues = append(r.Issues, issue("INVALID_VALUE", "financials.largest_customer_share_bps", "Доля должна быть от 0 до 10000 базисных пунктов"))
	}
	if len(r.Issues) > 0 {
		return r
	}
	if *c.HistoricalMaxDPD < 0 {
		r.Status = "UNSUPPORTED_INPUT"
		r.Issues = append(r.Issues, issue("INVALID_VALUE", "credit.historical_max_dpd_12m", "Просрочка не может быть отрицательной"))
		return r
	}
	dpd := *c.HistoricalMaxDPD
	payment := 0
	for i, v := range []int{0, 7, 29, 59, 89} {
		if dpd <= v {
			payment = []int{400, 320, 200, 80, 0}[i]
			break
		}
	}
	margin := 0
	for i, v := range []int{2000, 1500, 1000, 500, 0} {
		if f.MarginBPS >= v {
			margin = []int{350, 280, 210, 105, 35}[i]
			break
		}
	}
	concentration := 0
	for i, v := range []int{2000, 3500, 5000, 7500, 10000} {
		if *f.LargestCustomerShareBPS <= v {
			concentration = []int{250, 200, 125, 50, 0}[i]
			break
		}
	}
	r.Factors = []Factor{
		{ID: "payment_discipline", Value: dpd, Unit: "DAYS", WeightBPS: 4000, Points: payment, MaxPoints: 400, Status: "ASSESSED", Quality: "CALCULATED", ExplanationCode: "HISTORICAL_PAYMENT_DISCIPLINE", PeriodFrom: c.PeriodFrom, PeriodTo: c.PeriodTo, EvidenceRefs: []string{"CREDIT_HISTORY"}},
		{ID: "operating_margin", Value: f.MarginBPS, Unit: "BASIS_POINTS", WeightBPS: 3500, Points: margin, MaxPoints: 350, Status: "ASSESSED", Quality: "CALCULATED", ExplanationCode: "CFADS_TO_OPERATING_RECEIPTS", PeriodFrom: f.PeriodFrom, PeriodTo: f.PeriodTo, EvidenceRefs: []string{f.Provenance.SourceRef}},
		{ID: "customer_concentration", Value: *f.LargestCustomerShareBPS, Unit: "BASIS_POINTS", WeightBPS: 2500, Points: concentration, MaxPoints: 250, Status: "ASSESSED", Quality: "CALCULATED", ExplanationCode: "LARGEST_CUSTOMER_RECEIPTS_SHARE", PeriodFrom: f.PeriodFrom, PeriodTo: f.PeriodTo, EvidenceRefs: []string{f.Provenance.SourceRef}},
	}
	score := payment + margin + concentration
	r.Score = &score
	r.Grade = "E"
	r.Status = "ASSESSED"
	for i, v := range []int{800, 650, 500, 350, 0} {
		if score >= v {
			r.Grade = []string{"A", "B", "C", "D", "E"}[i]
			break
		}
	}
	return r
}
