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
	BingoRate float64 `json:"bingo_rate"`
	SaveRate  float64 `json:"save_rate"`
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

	var predTrueList []mysqlservice.PredTrue
	for t := tStart; t.Before(tEnd) || t.Equal(tEnd); t = t.Add(time.Minute) {
		predTrue, err := mysqlservice.GetBounceRecord("huadong", t.Format(layout))
		if err != nil {
			SendHttpResponse(w, &Response{
				StatusCode: 500,
				Message:    fmt.Sprintf("Get bounce record failed, err: %v", err),
				Data:       nil,
			}, http.StatusInternalServerError)
			return
		}
		predTrueList = append(predTrueList, *predTrue)
	}

	bingoNum := 0
	predSum := 0.0
	total := 1210.0

	for _, pt := range predTrueList {
		predSum += pt.Pred
		if pt.Pred >= float64(pt.True) {
			bingoNum++
		}
	}
	bingoRate := float64(bingoNum) / float64(len(predTrueList))
	saveRate := 1 - ((predSum / float64(len(predTrueList))) / total)

	SendHttpResponse(w, &Response{
		StatusCode: 0,
		Message:    "OK",
		Data: BounceRateResponse{
			BingoRate: bingoRate,
			SaveRate:  saveRate,
		},
	}, http.StatusOK)
}
