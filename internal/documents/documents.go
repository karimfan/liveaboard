package documents

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/store"
)

const MaxUploadBytes int64 = 10 << 20

var ErrInvalidFile = errors.New("documents: invalid file")

type Service struct {
	Dir string
}

type UploadInput struct {
	OrganizationID        uuid.UUID
	TripID                uuid.UUID
	TripGuestID           uuid.UUID
	UploadedByUserID      *uuid.UUID
	UploadedByGuestUserID *uuid.UUID
	Category              string
	DisplayName           string
	Notes                 *string
	File                  multipart.File
	Header                *multipart.FileHeader
}

type UploadResult struct {
	Document *store.GuestDocumentInput
	TempPath string
}

func New(dir string) *Service {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("var", "uploads", "guest-documents")
	}
	return &Service{Dir: dir}
}

func (s *Service) PrepareUpload(in UploadInput) (*UploadResult, error) {
	if in.File == nil || in.Header == nil {
		return nil, fmt.Errorf("%w: missing file", ErrInvalidFile)
	}
	category := strings.TrimSpace(in.Category)
	if !validCategory(category) {
		return nil, fmt.Errorf("%w: category", ErrInvalidFile)
	}
	original := safeFilename(in.Header.Filename)
	if original == "" {
		original = "document"
	}
	display := strings.TrimSpace(in.DisplayName)
	if display == "" {
		display = strings.TrimSuffix(original, filepath.Ext(original))
	}
	display = strings.TrimSpace(display)
	if display == "" {
		display = "Document"
	}
	declared := strings.ToLower(strings.TrimSpace(in.Header.Header.Get("Content-Type")))
	tmpDir := filepath.Join(s.Dir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(tmpDir, "upload-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer tmp.Close()

	hasher := sha256.New()
	limited := &io.LimitedReader{R: in.File, N: MaxUploadBytes + 1}
	n, err := io.Copy(io.MultiWriter(tmp, hasher), limited)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	if n <= 0 || n > MaxUploadBytes {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("%w: size", ErrInvalidFile)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	head := make([]byte, 512)
	readN, _ := io.ReadFull(tmp, head)
	if readN < len(head) {
		head = head[:readN]
	}
	contentType, err := validateContentType(head, declared, original)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	documentID := uuid.New()
	doc := &store.GuestDocumentInput{
		ID:                    documentID,
		OrganizationID:        in.OrganizationID,
		TripID:                in.TripID,
		TripGuestID:           in.TripGuestID,
		UploadedByUserID:      in.UploadedByUserID,
		UploadedByGuestUserID: in.UploadedByGuestUserID,
		Category:              category,
		DisplayName:           display,
		OriginalFilename:      original,
		ContentType:           contentType,
		SizeBytes:             n,
		SHA256Hex:             hex.EncodeToString(hasher.Sum(nil)),
		StorageKey:            StorageKey(in.OrganizationID, in.TripGuestID, documentID),
		Notes:                 in.Notes,
	}
	return &UploadResult{Document: doc, TempPath: tmpPath}, nil
}

func (s *Service) FinalizeUpload(tmpPath string, doc *store.GuestDocument) error {
	return s.FinalizePath(tmpPath, doc.StorageKey)
}

func (s *Service) FinalizePath(tmpPath, storageKey string) error {
	final := s.Path(storageKey)
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, final); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (s *Service) Path(storageKey string) string {
	return filepath.Join(s.Dir, filepath.FromSlash(storageKey))
}

func (s *Service) Remove(path string) {
	_ = os.Remove(path)
}

func StorageKey(orgID, tripGuestID, documentID uuid.UUID) string {
	return orgID.String() + "/" + tripGuestID.String() + "/" + documentID.String()
}

func IsBrowserInline(contentType string) bool {
	switch contentType {
	case "application/pdf", "image/jpeg", "image/png":
		return true
	default:
		return false
	}
}

func validateContentType(head []byte, declared, filename string) (string, error) {
	sniffed := http.DetectContentType(head)
	switch sniffed {
	case "application/pdf", "image/jpeg", "image/png":
		return sniffed, nil
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if (declared == "image/heic" || declared == "image/heif") && (ext == ".heic" || ext == ".heif") && hasHEIFBrand(head) {
		return declared, nil
	}
	return "", fmt.Errorf("%w: content_type", ErrInvalidFile)
}

func hasHEIFBrand(head []byte) bool {
	if len(head) < 12 {
		return false
	}
	return string(head[4:8]) == "ftyp" && (containsBrand(head, "heic") || containsBrand(head, "heix") || containsBrand(head, "hevc") || containsBrand(head, "hevx") || containsBrand(head, "mif1") || containsBrand(head, "msf1"))
}

func containsBrand(head []byte, brand string) bool {
	return strings.Contains(string(head), brand)
}

func validCategory(v string) bool {
	switch v {
	case "travel_document", "dive_certification", "dive_insurance", "liability_waiver", "medical", "other":
		return true
	default:
		return false
	}
}

func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "." || name == "/" {
		return ""
	}
	replacer := strings.NewReplacer("\x00", "", "\n", " ", "\r", " ", "\"", "")
	return replacer.Replace(name)
}
