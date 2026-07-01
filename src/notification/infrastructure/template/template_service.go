package template

import (
	"context"
	"fmt"
	"time"

	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/logger"

	"go.uber.org/zap"
)

type templateService struct {
	logger           *zap.Logger
	templateRepo     port.TemplateRepository
	fallbackToStatic bool // Fallback a mapeo estático si no hay DB
	contactEmail     string
}

// NewTemplateService crea el servicio de templates. contactEmail se inyecta desde env
// (CONTACT_EMAIL en main.go) — no se porta el config/viper del legacy, solo el valor que
// realmente usaba (Contact.Email) para no arrastrar una dependencia entera por un string.
func NewTemplateService(templateRepo port.TemplateRepository, contactEmail string) port.TemplateService {
	if contactEmail == "" {
		contactEmail = "contacto@mercadocercano.com"
	}
	return &templateService{
		logger:           logger.GetLogger(),
		templateRepo:     templateRepo,
		contactEmail:     contactEmail,
		fallbackToStatic: templateRepo == nil,
	}
}

func (s *templateService) RenderTemplateByAction(ctx context.Context, action domain.NotificationAction, notificationType domain.NotificationType, data map[string]interface{}) (subject string, html string, err error) {
	s.logger.Debug("Rendering template by action",
		zap.String("action", string(action)),
		zap.String("type", string(notificationType)))

	if data == nil {
		data = make(map[string]interface{})
	}
	data["current_year"] = time.Now().Year()
	data["contact_email"] = s.contactEmail

	template, err := s.GetTemplateByAction(ctx, action, notificationType)
	if err != nil {
		s.logger.Error("Failed to get template by action",
			zap.String("action", string(action)), zap.String("type", string(notificationType)), zap.Error(err))
		return "", "", err
	}

	html, err = template.RenderHTML(data)
	if err != nil {
		s.logger.Error("Failed to render template",
			zap.String("template_id", template.ID), zap.String("action", string(action)), zap.Error(err))
		return "", "", fmt.Errorf("error rendering template for action %s: %w", action, err)
	}

	subject = template.Subject
	return subject, html, nil
}

func (s *templateService) RenderTemplate(templateID string, data map[string]interface{}) (subject string, html string, err error) {
	s.logger.Debug("Rendering template", zap.String("template_id", templateID))

	if data == nil {
		data = make(map[string]interface{})
	}
	data["current_year"] = time.Now().Year()
	data["contact_email"] = s.contactEmail

	template := &domain.Template{
		ID:       templateID,
		Name:     templateID,
		Subject:  domain.GetTemplateSubject(templateID),
		FilePath: domain.GetTemplatePath(templateID),
	}

	html, err = template.RenderHTML(data)
	if err != nil {
		s.logger.Error("Failed to render template", zap.String("template_id", templateID), zap.Error(err))
		return "", "", fmt.Errorf("error rendering template %s: %w", templateID, err)
	}

	subject = template.Subject
	return subject, html, nil
}

func (s *templateService) GetTemplate(templateID string) (*domain.Template, error) {
	return &domain.Template{
		ID:       templateID,
		Name:     templateID,
		Subject:  domain.GetTemplateSubject(templateID),
		FilePath: domain.GetTemplatePath(templateID),
		Type:     domain.EmailNotification,
		IsActive: true,
	}, nil
}

func (s *templateService) GetTemplateByAction(ctx context.Context, action domain.NotificationAction, notificationType domain.NotificationType) (*domain.Template, error) {
	if !s.fallbackToStatic && s.templateRepo != nil {
		template, err := s.templateRepo.FindByAction(ctx, action, notificationType)
		if err != nil {
			s.logger.Warn("Template not found in database, falling back to static mapping",
				zap.String("action", string(action)), zap.Error(err))
			return s.getTemplateByActionStatic(action, notificationType)
		}
		return template, nil
	}
	return s.getTemplateByActionStatic(action, notificationType)
}

// getTemplateByActionStatic mapeo estático como fallback.
func (s *templateService) getTemplateByActionStatic(action domain.NotificationAction, notificationType domain.NotificationType) (*domain.Template, error) {
	mapping := domain.DefaultTemplateMapping()
	templateID, exists := mapping[action]
	if !exists {
		return nil, fmt.Errorf("no template found for action: %s", action)
	}

	return &domain.Template{
		ID:       templateID,
		Name:     templateID,
		Subject:  domain.GetTemplateSubject(templateID),
		FilePath: domain.GetTemplatePath(templateID),
		Action:   action,
		Type:     notificationType,
		IsActive: true,
	}, nil
}
