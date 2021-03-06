// Code generated by go-swagger; DO NOT EDIT.

package installer

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/openshift/assisted-service/models"
)

// UploadHostLogsNoContentCode is the HTTP code returned for type UploadHostLogsNoContent
const UploadHostLogsNoContentCode int = 204

/*UploadHostLogsNoContent Success.

swagger:response uploadHostLogsNoContent
*/
type UploadHostLogsNoContent struct {
}

// NewUploadHostLogsNoContent creates UploadHostLogsNoContent with default headers values
func NewUploadHostLogsNoContent() *UploadHostLogsNoContent {

	return &UploadHostLogsNoContent{}
}

// WriteResponse to the client
func (o *UploadHostLogsNoContent) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.Header().Del(runtime.HeaderContentType) //Remove Content-Type on empty responses

	rw.WriteHeader(204)
}

// UploadHostLogsNotFoundCode is the HTTP code returned for type UploadHostLogsNotFound
const UploadHostLogsNotFoundCode int = 404

/*UploadHostLogsNotFound Error.

swagger:response uploadHostLogsNotFound
*/
type UploadHostLogsNotFound struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewUploadHostLogsNotFound creates UploadHostLogsNotFound with default headers values
func NewUploadHostLogsNotFound() *UploadHostLogsNotFound {

	return &UploadHostLogsNotFound{}
}

// WithPayload adds the payload to the upload host logs not found response
func (o *UploadHostLogsNotFound) WithPayload(payload *models.Error) *UploadHostLogsNotFound {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the upload host logs not found response
func (o *UploadHostLogsNotFound) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UploadHostLogsNotFound) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(404)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// UploadHostLogsInternalServerErrorCode is the HTTP code returned for type UploadHostLogsInternalServerError
const UploadHostLogsInternalServerErrorCode int = 500

/*UploadHostLogsInternalServerError Error.

swagger:response uploadHostLogsInternalServerError
*/
type UploadHostLogsInternalServerError struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewUploadHostLogsInternalServerError creates UploadHostLogsInternalServerError with default headers values
func NewUploadHostLogsInternalServerError() *UploadHostLogsInternalServerError {

	return &UploadHostLogsInternalServerError{}
}

// WithPayload adds the payload to the upload host logs internal server error response
func (o *UploadHostLogsInternalServerError) WithPayload(payload *models.Error) *UploadHostLogsInternalServerError {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the upload host logs internal server error response
func (o *UploadHostLogsInternalServerError) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UploadHostLogsInternalServerError) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(500)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}
