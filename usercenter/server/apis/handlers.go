package apis

import (
	"log"
	"net/http"
	"time"
	"usercenter/config"
	"usercenter/database/model"
	"usercenter/database/service"
)

type DeviceLoginResponse struct {
	Instance *model.Instance `json:"instance"`
}

// 根据表单数据将终端接入可用实例
func DeviceLogin(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PostFormValue("zone_id")
	siteID := r.PostFormValue("site_id")
	deviceID := r.PostFormValue("device_id")

	if zoneID == "" || siteID == "" || deviceID == "" {
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, "Zone_id, site_id or device_id not specified")
		return
	}

	instance, err := service.GetInstanceAndLogin(zoneID, siteID, deviceID)
	if err != nil {
		if config.RECORDENABLED {
			service.InsertLoginFailure(zoneID, siteID, time.Now(), deviceID)
		}
		log.Printf("Failed to login: %v", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}

	SendHttpResponse(w, &Response{
		StatusCode: 200,
		Message:    "Succeeded to get available instance and login",
		Data: &DeviceLoginResponse{
			Instance: instance,
		},
	}, http.StatusOK)
}

// 根据表单数据将终端登出，修改 instance 为可用
func DeviceLogout(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PostFormValue("zone_id")
	deviceID := r.PostFormValue("device_id")

	if zoneID == "" || deviceID == "" {
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, "Zone_id or device_id not specified")
		return
	}

	err := service.LogoutDevice(zoneID, deviceID)
	if err != nil {
		log.Printf("Failed to logout: %v", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}

	SendHttpResponse(w, &Response{
		StatusCode: 200,
		Message:    "Device logout successfully",
	}, http.StatusOK)
}

// 心跳检测
func Healthz(w http.ResponseWriter, r *http.Request) {
	SendHttpResponse(w, &Response{
		StatusCode: 0,
		Message:    "OK",
		Data:       "Alive",
	}, http.StatusOK)
}
