package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	ocpp16 "github.com/lorenzodonini/ocpp-go/ocpp1.6"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/firmware"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/localauth"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/reservation"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/smartcharging"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
)

// Kommandolinjeparametre
var (
	chargePointID = flag.String("id", "CP001_SIMULATOR", "Charge Point ID")
	serverURL     = flag.String("url", "ws://localhost:8887/ocpp", "Central System WebSocket URL")
	model         = flag.String("model", "Simulator", "Charge Point model")
	vendor        = flag.String("vendor", "GoSimulator", "Charge Point vendor")
	heartbeatInt  = flag.Int("heartbeat", 60, "Heartbeat interval i sekunder")
)

// Hovedprogram
func main() {
	flag.Parse()

	log.Printf("Starter ladestander simulator med ID: %s", *chargePointID)
	log.Printf("Forbinder til: %s", *serverURL)

	// Opret OCPP charge point klient
	chargePoint := ocpp16.NewChargePoint(*chargePointID, nil, nil)

	// Opret og registrer handlers
	handler := &ChargePointHandler{
		chargePoint: chargePoint,
	}
	chargePoint.SetCoreHandler(handler)
	chargePoint.SetLocalAuthListHandler(handler)
	chargePoint.SetFirmwareManagementHandler(handler)
	chargePoint.SetReservationHandler(handler)
	chargePoint.SetRemoteTriggerHandler(handler)
	chargePoint.SetSmartChargingHandler(handler)

	// Opret kanal til at håndtere fejl
	errChan := chargePoint.Errors()
	go func() {
		for err := range errChan {
			log.Printf("OCPP fejl: %v", err)
		}
	}()

	// Forbind til central-system
	err := chargePoint.Start(*serverURL)
	if err != nil {
		log.Fatalf("Kunne ikke forbinde til central system: %v", err)
	}

	// Håndter CTRL+C for at afslutte pænt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Send boot notification
	log.Printf("Sender BootNotification...")
	bootConf, err := chargePoint.BootNotification(*model, *vendor)
	if err != nil {
		log.Fatalf("Boot notification fejlede: %v", err)
	}
	log.Printf("Boot bekræftet. Status: %v, Interval: %v", bootConf.Status, bootConf.Interval)

	// Send initial status notification
	sendStatusNotification(chargePoint, 0, core.ChargePointStatusAvailable)
	sendStatusNotification(chargePoint, 1, core.ChargePointStatusAvailable)
	sendStatusNotification(chargePoint, 2, core.ChargePointStatusAvailable)

	// Start heartbeat rutine
	heartbeatTicker := time.NewTicker(time.Duration(*heartbeatInt) * time.Second)
	defer heartbeatTicker.Stop()

	// Main loop - hold programmet kørende indtil CTRL+C
	for {
		select {
		case <-heartbeatTicker.C:
			// Send heartbeat
			conf, err := chargePoint.Heartbeat()
			if err != nil {
				log.Printf("Heartbeat fejlede: %v", err)
				continue
			}
			log.Printf("Heartbeat bekræftet. Server tid: %v", conf.CurrentTime.Format(time.RFC3339))

		case sig := <-sigChan:
			log.Printf("Modtaget signal: %v. Afslutter...", sig)
			chargePoint.Stop()
			return
		}
	}
}

// Helper funktion til at sende status notifikationer
func sendStatusNotification(chargePoint ocpp16.ChargePoint, connectorId int, status core.ChargePointStatus) {
	_, err := chargePoint.StatusNotification(
		connectorId,
		core.NoError,
		status,
	)
	if err != nil {
		log.Printf("Status notification fejlede for connector %d: %v", connectorId, err)
	} else {
		log.Printf("Status notification sendt for connector %d: %s", connectorId, status)
	}
}

// ChargePointHandler implementerer alle nødvendige handler interfaces
type ChargePointHandler struct {
	chargePoint ocpp16.ChargePoint
}

// ------------------- Core Profile Handlers -------------------
func (h *ChargePointHandler) OnChangeAvailability(request *core.ChangeAvailabilityRequest) (confirmation *core.ChangeAvailabilityConfirmation, err error) {
	log.Printf("ChangeAvailability modtaget: connector=%v, type=%v", request.ConnectorId, request.Type)
	// Simuler ændring af tilgængelighed
	go func() {
		if request.ConnectorId > 0 {
			sendStatusNotification(h.chargePoint, request.ConnectorId, core.ChargePointStatusUnavailable)
		} else {
			sendStatusNotification(h.chargePoint, 0, core.ChargePointStatusUnavailable)
			sendStatusNotification(h.chargePoint, 1, core.ChargePointStatusUnavailable)
			sendStatusNotification(h.chargePoint, 2, core.ChargePointStatusUnavailable)
		}
	}()
	return core.NewChangeAvailabilityConfirmation(core.AvailabilityStatusAccepted), nil
}

func (h *ChargePointHandler) OnChangeConfiguration(request *core.ChangeConfigurationRequest) (confirmation *core.ChangeConfigurationConfirmation, err error) {
	log.Printf("ChangeConfiguration modtaget: key=%v, value=%v", request.Key, request.Value)
	return core.NewChangeConfigurationConfirmation(core.ConfigurationStatusAccepted), nil
}

func (h *ChargePointHandler) OnClearCache(request *core.ClearCacheRequest) (confirmation *core.ClearCacheConfirmation, err error) {
	log.Printf("ClearCache modtaget")
	return core.NewClearCacheConfirmation(core.ClearCacheStatusAccepted), nil
}

func (h *ChargePointHandler) OnDataTransfer(request *core.DataTransferRequest) (confirmation *core.DataTransferConfirmation, err error) {
	log.Printf("DataTransfer modtaget: vendor=%v, messageId=%v", request.VendorId, request.MessageId)
	return core.NewDataTransferConfirmation(core.DataTransferStatusAccepted), nil
}

func getStringPtr(s string) *string {
	return &s
}

func (h *ChargePointHandler) OnGetConfiguration(request *core.GetConfigurationRequest) (confirmation *core.GetConfigurationConfirmation, err error) {
	log.Printf("GetConfiguration modtaget")
	// Simuler nogle konfigurationsnøgler
	var configKeys []core.ConfigurationKey
	if len(request.Key) == 0 {
		// Returner alle
		configKeys = []core.ConfigurationKey{
			{Key: "HeartbeatInterval", Value: getStringPtr(fmt.Sprintf("%d", *heartbeatInt)), Readonly: false},
			{Key: "ConnectionTimeOut", Value: getStringPtr("30"), Readonly: false},
			{Key: "MeterValueSampleInterval", Value: getStringPtr("60"), Readonly: false},
		}
	} else {
		// Returner specifikke nøgler
		for _, key := range request.Key {
			switch key {
			case "HeartbeatInterval":
				configKeys = append(configKeys, core.ConfigurationKey{Key: key, Value: getStringPtr(fmt.Sprintf("%d", *heartbeatInt)), Readonly: false})
			case "ConnectionTimeOut":
				configKeys = append(configKeys, core.ConfigurationKey{Key: key, Value: getStringPtr("30"), Readonly: false})
			case "MeterValueSampleInterval":
				configKeys = append(configKeys, core.ConfigurationKey{Key: key, Value: getStringPtr("60"), Readonly: false})
			default:
				// Ukendt nøgle
			}
		}
	}
	return core.NewGetConfigurationConfirmation(configKeys), nil
}

func (h *ChargePointHandler) OnRemoteStartTransaction(request *core.RemoteStartTransactionRequest) (confirmation *core.RemoteStartTransactionConfirmation, err error) {
	log.Printf("RemoteStartTransaction modtaget: idTag=%v", request.IdTag)
	// Simuler opstart af en transaktion
	go func() {
		connectorId := 1
		if request.ConnectorId != nil {
			connectorId = *request.ConnectorId
		}

		// Ændr status
		sendStatusNotification(h.chargePoint, connectorId, core.ChargePointStatusPreparing)
		time.Sleep(2 * time.Second)
		sendStatusNotification(h.chargePoint, connectorId, core.ChargePointStatusCharging)

		// Start transaktion
		txTime := types.NewDateTime(time.Now())
		_, err := h.chargePoint.StartTransaction(connectorId, request.IdTag, 0, txTime)
		if err != nil {
			log.Printf("Kunne ikke starte transaktion: %v", err)
		}
	}()
	return core.NewRemoteStartTransactionConfirmation(types.RemoteStartStopStatusAccepted), nil
}

func (h *ChargePointHandler) OnRemoteStopTransaction(request *core.RemoteStopTransactionRequest) (confirmation *core.RemoteStopTransactionConfirmation, err error) {
	log.Printf("RemoteStopTransaction modtaget: transactionId=%v", request.TransactionId)

	// Simuler stop af transaktion
	go func() {
		// Stop transaktion
		txTime := types.NewDateTime(time.Now())
		_, err := h.chargePoint.StopTransaction(1000, txTime, request.TransactionId)
		if err != nil {
			log.Printf("Kunne ikke stoppe transaktion: %v", err)
		}

		// Ændr status til ledig
		sendStatusNotification(h.chargePoint, 1, core.ChargePointStatusFinishing)
		time.Sleep(2 * time.Second)
		sendStatusNotification(h.chargePoint, 1, core.ChargePointStatusAvailable)
	}()

	return core.NewRemoteStopTransactionConfirmation(types.RemoteStartStopStatusAccepted), nil
}

func (h *ChargePointHandler) OnReset(request *core.ResetRequest) (confirmation *core.ResetConfirmation, err error) {
	log.Printf("Reset modtaget: type=%v", request.Type)
	return core.NewResetConfirmation(core.ResetStatusAccepted), nil
}

func (h *ChargePointHandler) OnUnlockConnector(request *core.UnlockConnectorRequest) (confirmation *core.UnlockConnectorConfirmation, err error) {
	log.Printf("UnlockConnector modtaget: connector=%v", request.ConnectorId)
	return core.NewUnlockConnectorConfirmation(core.UnlockStatusUnlocked), nil
}

// ------------------- LocalAuth Profile Handlers -------------------
func (h *ChargePointHandler) OnGetLocalListVersion(request *localauth.GetLocalListVersionRequest) (confirmation *localauth.GetLocalListVersionConfirmation, err error) {
	log.Printf("GetLocalListVersion modtaget")
	return localauth.NewGetLocalListVersionConfirmation(0), nil
}

func (h *ChargePointHandler) OnSendLocalList(request *localauth.SendLocalListRequest) (confirmation *localauth.SendLocalListConfirmation, err error) {
	log.Printf("SendLocalList modtaget: version=%v, updateType=%v", request.ListVersion, request.UpdateType)
	return localauth.NewSendLocalListConfirmation(localauth.UpdateStatusAccepted), nil
}

// ------------------- Firmware Profile Handlers -------------------
func (h *ChargePointHandler) OnGetDiagnostics(request *firmware.GetDiagnosticsRequest) (confirmation *firmware.GetDiagnosticsConfirmation, err error) {
	log.Printf("GetDiagnostics modtaget: location=%v", request.Location)

	// Simuler diagnostics upload
	go func() {
		// Skift status til uploading
		_, err := h.chargePoint.DiagnosticsStatusNotification(firmware.DiagnosticsStatusUploading)
		if err != nil {
			log.Printf("Kunne ikke sende diagnostics status notification: %v", err)
		}

		// Simuler upload tid
		time.Sleep(5 * time.Second)

		// Skift status til uploaded
		_, err = h.chargePoint.DiagnosticsStatusNotification(firmware.DiagnosticsStatusUploaded)
		if err != nil {
			log.Printf("Kunne ikke sende diagnostics status notification: %v", err)
		}
	}()

	return firmware.NewGetDiagnosticsConfirmation(), nil
}

func (h *ChargePointHandler) OnUpdateFirmware(request *firmware.UpdateFirmwareRequest) (confirmation *firmware.UpdateFirmwareConfirmation, err error) {
	log.Printf("UpdateFirmware modtaget: location=%v, retrieveDate=%v", request.Location, request.RetrieveDate)

	// Simuler firmware download og installation
	go func() {
		// Vent til det aftalte tidspunkt
		now := time.Now()
		retrieveTime := request.RetrieveDate.Time
		if retrieveTime.After(now) {
			waitTime := retrieveTime.Sub(now)
			log.Printf("Venter %v før firmware download startes", waitTime)
			time.Sleep(waitTime)
		}

		// Downloading
		_, err := h.chargePoint.FirmwareStatusNotification(firmware.FirmwareStatusDownloading)
		if err != nil {
			log.Printf("Kunne ikke sende firmware status notification: %v", err)
		}

		time.Sleep(5 * time.Second)

		// Downloaded
		_, err = h.chargePoint.FirmwareStatusNotification(firmware.FirmwareStatusDownloaded)
		if err != nil {
			log.Printf("Kunne ikke sende firmware status notification: %v", err)
		}

		time.Sleep(2 * time.Second)

		// Installing
		_, err = h.chargePoint.FirmwareStatusNotification(firmware.FirmwareStatusInstalling)
		if err != nil {
			log.Printf("Kunne ikke sende firmware status notification: %v", err)
		}

		time.Sleep(5 * time.Second)

		// Installed
		_, err = h.chargePoint.FirmwareStatusNotification(firmware.FirmwareStatusInstalled)
		if err != nil {
			log.Printf("Kunne ikke sende firmware status notification: %v", err)
		}
	}()

	return firmware.NewUpdateFirmwareConfirmation(), nil
}

// ------------------- Reservation Profile Handlers -------------------
func (h *ChargePointHandler) OnReserveNow(request *reservation.ReserveNowRequest) (confirmation *reservation.ReserveNowConfirmation, err error) {
	log.Printf("ReserveNow modtaget: connector=%v, expiryDate=%v, idTag=%v, reservationId=%v",
		request.ConnectorId, request.ExpiryDate, request.IdTag, request.ReservationId)
	return reservation.NewReserveNowConfirmation(reservation.ReservationStatusAccepted), nil
}

func (h *ChargePointHandler) OnCancelReservation(request *reservation.CancelReservationRequest) (confirmation *reservation.CancelReservationConfirmation, err error) {
	log.Printf("CancelReservation modtaget: reservationId=%v", request.ReservationId)
	return reservation.NewCancelReservationConfirmation(reservation.CancelReservationStatusAccepted), nil
}

// ------------------- Remote Trigger Profile Handlers -------------------
func (h *ChargePointHandler) OnTriggerMessage(request *remotetrigger.TriggerMessageRequest) (confirmation *remotetrigger.TriggerMessageConfirmation, err error) {
	log.Printf("TriggerMessage modtaget: requestedMessage=%v", request.RequestedMessage)

	// Håndter forskellige trigger typer
	go func() {
		switch request.RequestedMessage {
		case core.BootNotificationFeatureName:
			_, err := h.chargePoint.BootNotification(*model, *vendor)
			if err != nil {
				log.Printf("Triggered BootNotification fejlede: %v", err)
			}
		case core.HeartbeatFeatureName:
			_, err := h.chargePoint.Heartbeat()
			if err != nil {
				log.Printf("Triggered Heartbeat fejlede: %v", err)
			}
		case core.StatusNotificationFeatureName:
			connId := 0
			if request.ConnectorId != nil {
				connId = *request.ConnectorId
			}
			_, err := h.chargePoint.StatusNotification(connId, core.NoError, core.ChargePointStatusAvailable)
			if err != nil {
				log.Printf("Triggered StatusNotification fejlede: %v", err)
			}
		case firmware.DiagnosticsStatusNotificationFeatureName:
			_, err := h.chargePoint.DiagnosticsStatusNotification(firmware.DiagnosticsStatusIdle)
			if err != nil {
				log.Printf("Triggered DiagnosticsStatusNotification fejlede: %v", err)
			}
		case firmware.FirmwareStatusNotificationFeatureName:
			_, err := h.chargePoint.FirmwareStatusNotification(firmware.FirmwareStatusIdle)
			if err != nil {
				log.Printf("Triggered FirmwareStatusNotification fejlede: %v", err)
			}
		}
	}()

	return remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusAccepted), nil
}

// ------------------- Smart Charging Profile Handlers -------------------
func (h *ChargePointHandler) OnSetChargingProfile(request *smartcharging.SetChargingProfileRequest) (confirmation *smartcharging.SetChargingProfileConfirmation, err error) {
	log.Printf("SetChargingProfile modtaget: connector=%v, profileId=%v", request.ConnectorId, request.ChargingProfile.ChargingProfileId)
	return smartcharging.NewSetChargingProfileConfirmation(smartcharging.ChargingProfileStatusAccepted), nil
}

func (h *ChargePointHandler) OnClearChargingProfile(request *smartcharging.ClearChargingProfileRequest) (confirmation *smartcharging.ClearChargingProfileConfirmation, err error) {
	log.Printf("ClearChargingProfile modtaget")
	return smartcharging.NewClearChargingProfileConfirmation(smartcharging.ClearChargingProfileStatusAccepted), nil
}

func (h *ChargePointHandler) OnGetCompositeSchedule(request *smartcharging.GetCompositeScheduleRequest) (confirmation *smartcharging.GetCompositeScheduleConfirmation, err error) {
	log.Printf("GetCompositeSchedule modtaget: connector=%v, duration=%v", request.ConnectorId, request.Duration)
	return smartcharging.NewGetCompositeScheduleConfirmation(smartcharging.GetCompositeScheduleStatusAccepted), nil
}
