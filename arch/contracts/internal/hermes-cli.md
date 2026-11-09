# Внутренние интерфейсы hermes-cli

Статус: **проект интерфейсов**, до реализации и согласования окончательных сигнатур.
Здесь описаны Go-контракты между частями клиента.
Запросы, ответы и входящие сообщения вынесены во
[внешние интерфейсы](../external/hermes-cli.md).
Они уточняют [архитектуру клиента](../../hermes-cli.puml),
[адаптеры](../../hermes-cli-clients.puml) и [редактирование](../../hermes-cli-editing.puml).
[Схема интерфейсов](hermes-cli.puml) показывает их владельцев и реализации.

## Владение и зависимости

`internal/app` объявляет небольшие интерфейсы, необходимые его операциям.
HTTPDocumentClient **или** GRPCDocumentClient реализует AuthRemote, UsersRemote,
WorkspacesRemote и DocumentsRemote. HTTPNotificationClient **или** GRPCNotificationClient
может реализовать InboxRemote после согласования постоянных входящих.
Локальные адаптеры реализуют SessionStore, ConfigStore и DraftStore.
`internal/editor` объявляет Editor для presentation: прикладной слой не запускает редактор.

`main` собирает зависимости. CLI и TUI используют одни прикладные операции;
интерфейс для каждого прикладного struct вводить не требуется. Нет общего `Transport`
с методом `Send`: адаптеры предоставляют предметные операции, преобразуют DTO,
прокидывают context/deadline и возвращают общие модели/ошибки.

Ниже сигнатуры сведены в один Go-блок для проверки типов. При реализации интерфейсы
будут в пакетах потребителей, модели — в `internal/model`, адаптеры — отдельно.
Наличие модели в этом документе не выбирает формат сетевого протокола.

## Сигнатуры и модели

```go
package clicontracts

import (
	"context"
	"time"
)

type ID string
type Version int64
type Cursor string

type PageRequest struct {
	Cursor Cursor
	Limit  int
}
type Page[T any] struct {
	Items      []T
	NextCursor Cursor
}

// Immutable identity of the selected service environment, not a display label.
type ServerIdentity string
type SessionKey struct {
	Server ServerIdentity
	UserID ID
}
type Credentials struct {
	Method string
	Secret []byte // Method-specific input; never log or persist in drafts/config.
}
type Session struct {
	Key       SessionKey
	Token     string // Opaque bearer credential; do not log.
	ExpiresAt time.Time
}
type PublicProfile struct {
	ID          ID
	Username    string
	DisplayName string
}
type Profile struct {
	PublicProfile
	Email   string
	Version Version
}
type Workspace struct {
	ID          ID
	Name        string
	Description string
	Kind        string // personal | shared
	OwnerID     ID
	Version     Version
}
type Role string // owner | admin | editor | viewer; validated by the server.
type Membership struct {
	WorkspaceID ID
	UserID      ID
	Role        Role
}
type Invitation struct {
	ID          ID
	WorkspaceID ID
	InviterID   ID
	InviteeID   ID
	Role        Role
	Status      string // pending | accepted | declined | revoked | expired
	ExpiresAt   time.Time
}
type DocumentSummary struct {
	ID          ID
	WorkspaceID ID
	Title       string
	Version     Version
}
type Document struct {
	DocumentSummary
	Content string // Markdown in the first version.
}

// Protected adapters are bound to one SessionKey at construction. They load
// its token through SessionStore; the request never supplies an actor user ID.
type AuthRemote interface {
	Authenticate(context.Context, Credentials) (Session, error)
	Logout(context.Context, Session) error
}
type RegisterInput struct {
	Username    string
	DisplayName string
	Email       string
	Credentials Credentials
}
type Registration struct {
	Profile           Profile
	PersonalWorkspace Workspace
}
type ProfileChanges struct {
	Username        *string
	DisplayName     *string
	ExpectedVersion Version
}
type EmailChangeInput struct {
	NewEmail        string
	ExpectedVersion Version
}
type IdentityConfirmation struct {
	Method string
	Proof  []byte // Selected reauthentication method remains OPEN.
}
type DeleteAccountInput struct {
	ExpectedVersion Version
	Confirmation    IdentityConfirmation
}
type UsersRemote interface {
	Register(context.Context, RegisterInput) (Registration, error)
	GetSelf(context.Context) (Profile, error)
	GetPublicProfile(context.Context, ID) (PublicProfile, error)
	UpdateSelf(context.Context, ProfileChanges) (Profile, error)
	RequestEmailChange(context.Context, EmailChangeInput) error
	ConfirmEmail(context.Context, string) (Profile, error)
	GetDeletionBlockers(context.Context) ([]Workspace, error)
	DeleteSelf(context.Context, DeleteAccountInput) error
}

type CreateWorkspaceInput struct {
	Name        string
	Description string
}
type WorkspaceChanges struct {
	Name            *string
	Description     *string
	ExpectedVersion Version
}
type InviteMemberInput struct {
	InviteeID ID
	Role      Role
}
type InvitationDecision string // accept | decline
// Explicit confirmation is prepared by presentation; server rechecks it.
type DeleteInput struct {
	ExpectedVersion Version
	Confirmed       bool
}
type WorkspacesRemote interface {
	CreateWorkspace(context.Context, CreateWorkspaceInput) (Workspace, error)
	ListWorkspaces(context.Context, PageRequest) (Page[Workspace], error)
	GetWorkspace(context.Context, ID) (Workspace, error)
	UpdateWorkspace(context.Context, ID, WorkspaceChanges) (Workspace, error)
	ListMembers(context.Context, ID, PageRequest) (Page[Membership], error)
	ListInvitations(context.Context, PageRequest) (Page[Invitation], error)
	InviteMember(context.Context, ID, InviteMemberInput) (Invitation, error)
	RespondToInvitation(context.Context, ID, InvitationDecision) (Invitation, error)
	RevokeInvitation(context.Context, ID) error
	ChangeRole(context.Context, ID, ID, Role) (Membership, error)
	RemoveMember(context.Context, ID, ID) error
	LeaveWorkspace(context.Context, ID) error
	TransferOwnership(context.Context, ID, ID, Version) (Workspace, error)
	DeleteWorkspace(context.Context, ID, DeleteInput) error
}

type DocumentQuery struct {
	WorkspaceID *ID // nil: all currently accessible workspaces.
	Search      string
	Page        PageRequest
}
type CreateDocumentInput struct {
	WorkspaceID ID
	Title       string
	Content     string
}
type UpdateDocumentInput struct {
	Title           string
	Content         string
	ExpectedVersion Version
}
type DocumentWrite struct {
	Document Document
	Changed  bool // false means confirmed no-op; not a second mutation.
}
type Subscription struct {
	DocumentID ID
	Enabled    bool
}
type DocumentsRemote interface {
	Create(context.Context, CreateDocumentInput) (Document, error)
	List(context.Context, DocumentQuery) (Page[DocumentSummary], error)
	Get(context.Context, ID) (Document, error)
	Update(context.Context, ID, UpdateDocumentInput) (DocumentWrite, error)
	Delete(context.Context, ID, DeleteInput) error
	Subscribe(context.Context, ID) (Subscription, error)
	Unsubscribe(context.Context, ID) (Subscription, error)
}

// Load reports missing session as CodeNotFound; an expired local timestamp
// does not imply confirmed server revocation. Key/token comparisons are atomic.
type SessionStore interface {
	Load(context.Context, SessionKey) (Session, error)
	Save(context.Context, Session) error
	DeleteIfCurrent(context.Context, SessionKey, string) error
}
type ClientConfig struct {
	Server      ServerIdentity
	UserID      ID
	WorkspaceID *ID
}
type ConfigStore interface {
	Load(context.Context) (ClientConfig, error)
	Save(context.Context, ClientConfig) error
}

type DraftID string
type DraftScope struct {
	SessionKey SessionKey
}
type Draft struct {
	ID              DraftID
	Scope           DraftScope
	Base            Document // Original title/content/version retained for comparison.
	WorkingFilePath string
	ProposedTitle   string
}
type WorkingCopy struct {
	Title   string
	Content string
}
type DraftStore interface {
	Create(context.Context, DraftScope, Document) (Draft, error)
	Load(context.Context, DraftScope, DraftID) (Draft, error)
	List(context.Context, DraftScope) ([]Draft, error)
	ReadWorking(context.Context, DraftScope, DraftID) (WorkingCopy, error)
	SetTitle(context.Context, DraftScope, DraftID, string) error
	SetBase(context.Context, DraftScope, DraftID, Document) (Draft, error)
	Delete(context.Context, DraftScope, DraftID) error
}

// Owned by internal/editor and consumed by CLI/TUI, not by internal/app.
type Editor interface {
	Edit(context.Context, string) error // Existing persistent working-file path.
}

// Result models for concrete internal/app.Documents methods, not a new adapter.
type SaveOutcome string

const (
	Saved     SaveOutcome = "saved"
	Unchanged SaveOutcome = "unchanged"
	Conflict  SaveOutcome = "conflict"
	Unknown   SaveOutcome = "unknown"
	Failed    SaveOutcome = "failed"
)

type SaveResult struct {
	Outcome      SaveOutcome
	Document     *Document
	RecoveryPath string
	Cause        error
	CleanupError error
}
type Comparison struct {
	Base   Document
	Local  WorkingCopy
	Latest Document
}

type ErrorCode string

const (
	CodeValidation      ErrorCode = "validation"
	CodeUnauthenticated ErrorCode = "unauthenticated"
	CodeDenied          ErrorCode = "denied"
	CodeNotFound        ErrorCode = "not_found"
	CodeConflict        ErrorCode = "conflict"
	CodeUnavailable     ErrorCode = "unavailable"
	CodeOutcomeUnknown  ErrorCode = "outcome_unknown"
	CodeLocalIO         ErrorCode = "local_io"
)

type ContractError interface {
	error
	Code() ErrorCode
	Unwrap() error
}
```

Повторяющиеся ID в методах пространств имеют порядок: workspace ID, затем user ID.
У `TransferOwnership` последний аргумент — ожидаемая версия пространства.
Имена CRUD-методов WorkspacesRemote содержат Workspace, чтобы один Go-адаптер
мог одновременно реализовать DocumentsRemote с его Get/Create/Update/Delete.
`ListInvitations` возвращает приглашения текущего пользователя, чтобы их можно было
найти независимо от уведомлений. Поля `*string` различают отсутствие изменения и
явно заданную пустую строку. Значения ролей, решений и подтверждений сервер проверяет
повторно; типы не заменяют авторизацию.

## Сессия и конфигурация

- При входе Auth сохраняет Session в SessionStore; несохранённый вход не считается
  завершённым. При ошибке локальной записи предпринимается отзыв полученного токена.
- Logout принимает **конкретную** Session, а не перечитывает текущий токен адаптера.
  Так очистка не отзовёт более новый вход. Отсутствующая/истёкшая сессия — успешный
  серверный logout, недоступность Redis — ошибка, а не подтверждение отзыва.
- `DeleteIfCurrent` атомарно удаляет только совпавшую запись; несовпадение/отсутствие —
  no-op. Unauthenticated очищает только токен, использованный отвергнутым запросом.
  Denied, сетевой сбой и таймаут не удаляют сессию.
- SessionStore — защищённое локальное хранилище; ConfigStore не содержит токенов.
  ConfigStore.Save атомарно заменяет локальную конфигурацию; ошибка сохраняет прежнюю.
  Политика конкурирующих изменений несколькими процессами пока не определена.
- Выбор пространства локальный: он сохраняется после успешного чтения, но не даёт
  прав. Явный workspace параметр операции имеет приоритет над ConfigStore.
- Конкретный способ credentials, подтверждения личности и срок сессии открыты.
  Credentials/IdentityConfirmation — временные входные данные, не файлы конфигурации.
- Успешный RequestEmailChange означает сохранённый запрос подтверждения, а не смену
  email. Успешный DeleteSelf означает подтверждённые удаление и отзыв сессий; при
  частичном/неизвестном исходе адаптер возвращает ошибку, а не nil.

## Черновики и редактор

`DraftStore` не отправляет данные на сервер. `Create` сохраняет базовую версию и
рабочий Markdown-файл до запуска редактора, выдаёт уникальный DraftID и путь.
Неуспех не возвращает готовый черновик. Файлы доступны только владельцу.
Все операции проверяют scope: сервер, пользователь и документ не подменяются.
`List` позволяет восстановить черновики после перезапуска; `ReadWorking` читает
изменённый редактором текст и ProposedTitle. `SetTitle` меняет только предлагаемый
заголовок; `SetBase` — только базовый снимок того же документа, сохраняя локальные правки.
При ошибке изменения метаданных прежнее состояние остаётся читаемым.
`Delete` идемпотентен для отсутствующего черновика своего scope; ошибки удаления
передаются вызывающему, а записи чужого scope не затрагиваются.

Конкретный `internal/app.Documents` предоставляет следующие операции из схемы
редактирования; здесь они не объявляются ещё одним интерфейсом:

| Операция | Результат и условие |
| --- | --- |
| `BeginEdit(ctx, documentID)` | Draft после Get и подтверждённого DraftStore.Create |
| `SaveEdit(ctx, draftID)` | SaveResult; запись с Base.Version через DocumentsRemote.Update |
| `CompareLatest(ctx, draftID)` | Comparison после нового защищённого Get; сравнение не меняет базу |
| `RebaseDraft(ctx, draftID, reviewedVersion)` | Draft; применить только снимок, который пользователь просмотрел, с совпадающей версией; без скрытой подстановки более свежего |
| `DiscardDraft(ctx, draftID)` | error; удалить через DraftStore после явного решения пользователя |

Scope берётся из выбранного аккаунта, сверяется с черновиком и остаётся фиксированным
на время операции. Rebase не является автоматическим merge; способ хранения
просмотренного снимка внутри Documents предстоит уточнить.

Editor.Edit получает существующий файл и не удаляет его после отмены/ошибки.
CLI запускает редактор на переднем плане; TUI приостанавливает и восстанавливает
терминал. Приложение не читает stdin и не рисует экраны.

| SaveResult.Outcome | Поведение |
| --- | --- |
| saved / unchanged | Сохранение подтверждено; можно удалить черновик. Ошибка очистки идёт отдельно в CleanupError и RecoveryPath |
| conflict | Сохранить файл; предложить CompareLatest и явный RebaseDraft |
| unknown | Сервер мог записать изменение; сохранить файл, не повторять запись автоматически |
| failed | Ошибка валидации/доступа/локального чтения либо подтверждённый отказ; сохранить существующий черновик |

Document заполняется для saved/unchanged, Cause — для conflict/unknown/failed.
Сбой очистки не меняет saved на failed. Успешный Get после таймаута помогает
сравнить состояние, но совпавший текст сам по себе не доказывает исход записи.

## Ошибки и context

Адаптеры возвращают ContractError с кодом и безопасным контекстом, сохраняя исходную
ошибку через Unwrap. Ошибки не содержат bearer token, credentials или тело документа.
Отмена и deadline распознаются через `errors.Is(err, context.Canceled/DeadlineExceeded)`.
Для мутации, которая могла дойти до сервера, отмена/сетевой сбой может одновременно
означать outcome_unknown: deadline не доказывает rollback.

Версионный конфликт не повторяется с новой версией автоматически. Отсутствие или
недоступность объекта может отображаться единым not_found без раскрытия его существования.
Доступ к целевому объекту каждый раз проверяет владеющий им сервер, не уведомление.
Конкретное отображение HTTP/gRPC-кодов здесь не фиксируется.

## Уведомления: отдельный следующий контракт

В архитектуре есть InboxRemote, но сценарии больше не предполагают постоянную историю
всех live-сообщений. Поэтому его окончательную сигнатуру пока не закрепляем:

| Кандидат | Что нужно определить |
| --- | --- |
| InboxRemote: List / UnreadCount / MarkRead / MarkAllRead | Нужна ли постоянная история; модели, фильтры, курсор, видимость и граница mark-all |
| LiveRemote: Connect / Receive / Close | Модель сигнала, context/lifecycle, reconnect, backpressure и ошибки |
| Восстановление offline-сообщений | Ack, claim одной сессией, повторы, момент удаления и восстановление после обрыва |
| SettingsRemote | Владение и операции настроек после согласования ADR-003 |

LiveRemote не объединяется с InboxRemote и не выдаётся за HTTP/gRPC-адаптер.
Сообщение содержит ссылку/факт; открытие документа идёт через DocumentsRemote.Get.
Выбранный wire-протокол WebSocket, Kafka event envelope и Redis key schema не являются
внутренними интерфейсами CLI и описываются отдельно, если пересекают внешнюю границу клиента.
Проект таких сообщений: [внешние интерфейсы](../external/hermes-cli.md).
