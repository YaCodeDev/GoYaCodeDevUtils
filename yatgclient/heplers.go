package yatgclient

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gotd/td/tg"
)

// UploadMediaPhoto uploads a photo file and returns an InputMediaPhoto
// for use in messages.
//
// Example usage:
//
//	photoFile, _ := os.Open("path/to/photo.jpg")
//
//	inputMediaPhoto, err := client.UploadMediaPhoto(ctx, photoFile)
//	if err != nil {
//	    // Handle error
//	}
func (c *Client) UploadMediaPhoto(
	ctx context.Context,
	file io.Reader,
) (*tg.InputMediaPhoto, yaerrors.Error) {
	uploadedMedia, err := c.UploadFile(ctx, file)
	if err != nil {
		return nil, err
	}

	uploadedPhoto, ok := uploadedMedia.(*tg.MessageMediaPhoto)
	if !ok {
		return nil, yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"failed to upload photo",
			c.log,
		)
	}

	photo, ok := uploadedPhoto.Photo.(*tg.Photo)
	if !ok {
		return nil, yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"uploaded media is not a photo",
			c.log,
		)
	}

	inputPhotoClass := &tg.InputPhoto{
		ID:         photo.GetID(),
		AccessHash: photo.AccessHash,
	}

	return &tg.InputMediaPhoto{
		ID: inputPhotoClass,
	}, nil
}

// UploadMediaDocument uploads a document file and returns an InputMediaDocument
// for use in messages.
//
// Example usage:
//
//	documentFile, _ := os.Open("path/to/document.pdf")
//
//	inputMediaDocument, err := client.UploadMediaDocument(ctx, documentFile)
//	if err != nil {
//	    // Handle error
//	}
func (c *Client) UploadMediaDocument(
	ctx context.Context,
	file io.Reader,
) (*tg.InputMediaDocument, yaerrors.Error) {
	uploadedMedia, err := c.UploadFile(ctx, file)
	if err != nil {
		return nil, err
	}

	uploadedDocument, ok := uploadedMedia.(*tg.MessageMediaDocument)
	if !ok {
		return nil, yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"failed to upload document",
			c.log,
		)
	}

	document, ok := uploadedDocument.Document.(*tg.Document)
	if !ok {
		return nil, yaerrors.FromStringWithLog(
			http.StatusInternalServerError,
			"uploaded media is not a document",
			c.log,
		)
	}

	inputDocumentClass := &tg.InputDocument{
		ID:         document.GetID(),
		AccessHash: document.AccessHash,
	}

	return &tg.InputMediaDocument{
		ID: inputDocumentClass,
	}, nil
}

// UploadFile uploads a file in chunks and returns the uploaded media.
//
// Example usage:
//
//	file, _ := os.Open("path/to/file")
//
//	uploadedMedia, err := client.UploadFile(ctx, file)
//	if err != nil {
//	    // Handle error
//	}
func (c *Client) UploadFile(
	ctx context.Context,
	file io.Reader,
) (tg.MessageMediaClass, yaerrors.Error) {
	var (
		peer    tg.InputPeerClass
		chunkID int
	)

	randID := rand.Int63()

	buf := make([]byte, c.chunkSize)
	for {
		n, err := io.ReadFull(file, buf)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, yaerrors.FromErrorWithLog(
					http.StatusInternalServerError,
					err,
					"failed to read file chunk",
					c.log,
				)
			}

			buf = buf[:n]
		}

		if n == 0 {
			break
		}

		_, err = c.API().UploadSaveFilePart(ctx, &tg.UploadSaveFilePartRequest{
			FileID:   randID,
			FilePart: chunkID,
			Bytes:    buf,
		})
		if err != nil {
			return nil, yaerrors.FromErrorWithLog(
				http.StatusInternalServerError,
				err,
				"failed to upload file part",
				c.log,
			)
		}

		chunkID++
	}

	if c.IsBot {
		peer = &tg.InputPeerEmpty{}
	} else {
		peer = &tg.InputPeerSelf{}
	}

	uploadedMedia, serr := c.API().MessagesUploadMedia(ctx, &tg.MessagesUploadMediaRequest{
		Peer: peer,
		Media: &tg.InputMediaUploadedPhoto{
			File: &tg.InputFile{
				ID:    randID,
				Parts: chunkID,
			},
		},
	})
	if serr != nil {
		return nil, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			serr,
			"failed to upload media",
			c.log,
		)
	}

	return uploadedMedia, nil
}
