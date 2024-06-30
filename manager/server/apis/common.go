package apis

import (
	"fmt"
	mysqlservice "manager/mysql/service"
	"net/http"
	"time"
)

func Healthz(w http.ResponseWriter, r *http.Request) {
	SendHttpResponse(w, &Response{
		StatusCode: 200,
		Message:    "OK",
		Data:       "Alive",
	}, http.StatusOK)
}

type BounceRateResponse struct {
	TrueIns   []float64 `json:"true_ins"`
	BounceIns []float64 `json:"bounce_ins"`
}

func BounceRate(w http.ResponseWriter, r *http.Request) {
	// 通过 Query 的形式传递参数。
	query := r.URL.Query()
	layout := "2006-01-02 15:04:00"
	// 参数格式："2006-01-02 15:04:00"，必须要保证秒数是 0。
	start := query.Get("start")
	end := query.Get("end")

	tStart, err := time.Parse(layout, start)
	if err != nil {
		SendHttpResponse(w, &Response{
			StatusCode: 400,
			Message:    "Invalid start time",
			Data:       nil,
		}, http.StatusBadRequest)
		return
	}
	tEnd, err := time.Parse(layout, end)
	if err != nil {
		SendHttpResponse(w, &Response{
			StatusCode: 400,
			Message:    "Invalid end time",
			Data:       nil,
		}, http.StatusBadRequest)
		return
	}

	predTrueList, err := mysqlservice.GetBounceRecords("huadong", tStart.Format(layout), tEnd.Format(layout))
	if err != nil {
		SendHttpResponse(w, &Response{
			StatusCode: 500,
			Message:    fmt.Sprintf("Get bounce record failed, err: %v", err),
			Data:       nil,
		}, http.StatusInternalServerError)
		return
	}

	var TrueIns, BounceIns []float64
	for _, pt := range predTrueList {
		TrueIns = append(TrueIns, float64(pt.True))
		BounceIns = append(BounceIns, pt.Pred)
	}

	SendHttpResponse(w, &Response{
		StatusCode: 0,
		Message:    "OK",
		Data: BounceRateResponse{
			TrueIns:   TrueIns,
			BounceIns: BounceIns,
		},
	}, http.StatusOK)
}
