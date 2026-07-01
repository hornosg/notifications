package email

import (
	"context"

	appctx "notifications/src/notification/application/appcontext"
	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/logger"

	"notifications/pkg/validator"

	"github.com/resendlabs/resend-go"
	"go.uber.org/zap"
)

type resendClient struct {
	client            *resend.Client
	logger            *zap.Logger
	fromEmail         string
	templateService   port.TemplateService
	projectConfigRepo port.ProjectConfigRepository
}

// NewResendClient crea el sender de emails. Si se provee projectConfigRepo, from_email/from_name
// se resuelven por namespace desde la DB; de lo contrario se usa el fallback de env vars.
func NewResendClient(apiKey string, fromEmail string, templateService port.TemplateService, projectConfigRepo port.ProjectConfigRepository) port.EmailSender {
	return &resendClient{
		client:            resend.NewClient(apiKey),
		logger:            logger.GetLogger(),
		fromEmail:         fromEmail,
		templateService:   templateService,
		projectConfigRepo: projectConfigRepo,
	}
}

// resolveFrom retorna el remitente para el namespace del contexto (appctx.WithRLS, armado
// por middleware.ContextWithRLSFromGin desde el JWT ya validado). Fallback a fromEmail si
// no hay repo o no hay config para ese namespace.
func (client *resendClient) resolveFrom(ctx context.Context) (from string, fromName string) {
	if client.projectConfigRepo == nil {
		return client.fromEmail, ""
	}

	namespace := appctx.NamespaceFromContext(ctx)
	cfg, err := client.projectConfigRepo.FindByNamespace(ctx, namespace)
	if err != nil {
		client.logger.Warn("Failed to resolve project config for sender identity, falling back to env vars",
			zap.String("namespace", namespace), zap.Error(err))
		return client.fromEmail, ""
	}

	return cfg.FromEmail, cfg.FromName
}

func (client *resendClient) SendEmail(ctx context.Context, to string, templateID string, data map[string]interface{}) error {
	client.logger.Info("Sending email", zap.String("to", to), zap.String("template_id", templateID))

	subject, html, err := client.templateService.RenderTemplate(templateID, data)
	if err != nil {
		client.logger.Error("Failed to render template", zap.String("template_id", templateID), zap.Error(err))
		return err
	}

	return client.sendEmail(ctx, to, subject, html)
}

func (client *resendClient) SendEmailByAction(ctx context.Context, to string, action domain.NotificationAction, notificationType domain.NotificationType, data map[string]interface{}) error {
	client.logger.Info("Sending email by action",
		zap.String("to", to), zap.String("action", string(action)), zap.String("type", string(notificationType)))

	subject, html, err := client.templateService.RenderTemplateByAction(ctx, action, notificationType, data)
	if err != nil {
		client.logger.Error("Failed to render template by action",
			zap.String("action", string(action)), zap.String("type", string(notificationType)), zap.Error(err))
		return err
	}

	return client.sendEmail(ctx, to, subject, html)
}

func (client *resendClient) sendEmail(ctx context.Context, to, subject, html string) error {
	from, fromName := client.resolveFrom(ctx)
	if from == "" {
		client.logger.Error("No from email configured")
		return &ResendError{Message: "from_email not configured"}
	}

	displayFrom := from
	if fromName != "" {
		displayFrom = fromName + " <" + from + ">"
	}

	params := &resend.SendEmailRequest{
		From:    displayFrom,
		To:      []string{to},
		Subject: subject,
		Html:    html,
	}

	_, err := client.client.Emails.Send(params)
	if err != nil {
		client.logger.Error("Failed to send email", zap.String("to", to), zap.String("subject", subject), zap.Error(err))
		return err
	}

	client.logger.Info("Email sent successfully", zap.String("to", to), zap.String("subject", subject))
	return nil
}

func (client *resendClient) ValidateEmail(email string) bool {
	return validator.IsValidEmail(email)
}

// ResendError es un error explícito del sender para facilitar testing/mapeo HTTP.
type ResendError struct {
	Message string
}

func (e *ResendError) Error() string {
	return e.Message
}
