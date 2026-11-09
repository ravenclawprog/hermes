# Внешние интерфейсы hermes-cli: сообщения

Статус: **проект сообщений для обсуждения**, основанный на
[сценариях](../../scenarios/README.md). Форматы и имена ниже предложены;
принятые направления и открытые гарантии определяются [ADR](../../decisions/README.md).
[Схема сообщений](hermes-cli.puml). [Внутренние Go-интерфейсы](../internal/hermes-cli.md).

Здесь «внешний интерфейс» — запрос, ответ или сообщение, пересекающее границу
hermes-cli. Клиент отправляет команды и запросы document-service, получает ответы,
открывает WebSocket к notification-service и принимает live/offline-сообщения.
Клиент не подключается к PostgreSQL, Redis или Kafka и не получает Kafka DomainEvent
как публичное сообщение автоматически: HNS преобразует предметные события по политикам.

## Общие правила сообщений

Описываем логический контракт до выбора HTTP/gRPC. JSON ниже — читаемая иллюстрация
предлагаемой формы, не выбор REST, endpoint или protobuf. Для HTTP/gRPC адаптер
преобразует логический конверт в body/headers/metadata и response/status.

| Поле запроса | Тип | Назначение |
| --- | --- | --- |
| schema_version | integer, сейчас 1 | Версия предлагаемой схемы сообщения |
| request_id | string | Корреляция одного запроса и ответа; не ключ идемпотентности |
| operation | string | Имя операции из таблиц ниже |
| payload | object | Поля конкретного запроса; `{}` для пустого запроса |

Защищённые запросы передают один opaque session token через защищённые
headers/metadata, отдельно от payload; конкретное имя заголовка/metadata ещё не выбрано.
Токен не попадает в URL, логи или диагностические сообщения. Actor определяется
сервером по сессии: клиент не назначает себе user_id или роль в запросе.
Исключение Logout — передаётся именно отзываемый токен, а не токен более нового входа.

| Поле ответа | Тип | Назначение |
| --- | --- | --- |
| schema_version | integer | Версия схемы ответа |
| request_id | string | request_id исходного запроса |
| operation | string | Имя исходной операции |
| status | success / error | Результат обработки |
| payload | object | Только у success, результат из таблицы |
| error | Error | Только у error; одновременно payload и error не передаются |

Примитивы: `ID` — строковый непрозрачный идентификатор, не имя; `Version` — положительное
целое; `Timestamp` — RFC 3339 UTC; `Cursor` — непрозрачная строка сервера.
Выбор UUID-формата, числового представления int64 и совместимости схем предстоит
зафиксировать при выборе wire encoding. Нельзя преобразовывать int64 с потерей точности.
`?` означает необязательное поле; отсутствие отличается от пустого значения.

`PageInput = {cursor?: Cursor, limit?: integer}`: первый запрос без cursor;
положительный limit, значение по умолчанию и максимум OPEN. При продолжении фильтры
сохраняются. `Page<T> = {items: T[], next_cursor?: Cursor}`; отсутствие next_cursor
означает конец выдачи. Конкурентные изменения и стабильность снимка требуют решения.

## Общие объекты в ответах

| Объект | Поля |
| --- | --- |
| PublicProfile | id: ID, username: string, display_name: string |
| Profile | поля PublicProfile + email: string, version: Version; только владельцу |
| Workspace | id: ID, name: string, description: string, kind: personal/shared, owner_id: ID, version: Version |
| Membership | workspace_id: ID, user_id: ID, role: owner/admin/editor/viewer |
| Invitation | id: ID, workspace_id: ID, inviter_id: ID, invitee_id: ID, role: admin/editor/viewer, status: pending/accepted/declined/revoked/expired, expires_at: Timestamp |
| DocumentSummary | id: ID, workspace_id: ID, title: string, version: Version |
| Document | поля DocumentSummary + content: string (Markdown) |
| Subscription | document_id: ID, enabled: boolean |
| Credentials | method: string, proof: object; поля proof OPEN, зависят от выбранной аутентификации |
| IdentityConfirmation | method: string, proof: object; подтверждение личности для опасной операции, формат OPEN |

Это DTO сообщения, а не сериализация внутренних структур Go.
Credentials.Secret, SessionKey, ServerIdentity, Draft, RecoveryPath, локальные пути,
context.Context и Go error через сеть не передаются. ServerIdentity задаётся локальной
конфигурацией; UserID/Token/ExpiresAt ответа Authenticate адаптер преобразует в Session.
Пустой изменяемый заголовок и имя валидируются сервером; отсутствие patch-поля означает
«не изменять». Сервер не возвращает credentials/confirmation proof в профилях.

## CLI → document-service: пользователи и сессия

В столбце «Ответ» указан payload успешного ответа. Вместо него каждая операция может
вернуть Error. Названия согласованы с use case в сценариях; имена методов Go могут
отличаться (например, DocumentsRemote.Get → GetDocument).

| Запрос | Payload запроса | Ответ | Сессия |
| --- | --- | --- | --- |
| Register | username, display_name, email: string; credentials: Credentials | profile: Profile, personal_workspace: Workspace | Нет |
| Authenticate | credentials: Credentials | user_id: ID, token: string, expires_at: Timestamp | Нет |
| Logout | {} | {} после подтверждённого отзыва/отсутствия сессии | Отзываемый токен; отсутствие/истечение допустимо |
| GetSelf | {} | profile: Profile | Да |
| GetPublicProfile | user_id: ID целевого пользователя | profile: PublicProfile | Да |
| UpdateSelf | username?: string, display_name?: string, expected_version: Version | profile: Profile | Да |
| RequestEmailChange | new_email: string, expected_version: Version | confirmation_required: true | Да |
| ConfirmEmail | confirmation_token: string | profile: Profile | Одноразовый confirmation token; повторный session token не предполагается |
| GetDeletionBlockers | {} | workspaces: Workspace[] | Да |
| DeleteSelf | expected_version: Version, confirmation: IdentityConfirmation | {} только после подтверждённого удаления и отзыва сессий | Да |

Регистрация атомарно создаёт пользователя, личное пространство и owner membership,
но не выполняет вход. Способ передачи/подтверждения credentials и исходная email
verification OPEN. GetPublicProfile ограничен общей активной membership.
RequestEmailChange сохраняет запрос, а не меняет email. ConfirmEmail меняет email
только после проверки одноразового секрета, срока и уникальности; секрет не логируется.
Предварительные deletion blockers не отменяют повторных проверок при DeleteSelf.
Частичный/неизвестный результат PostgreSQL + Redis не выдаётся за success.

## CLI → document-service: пространства и участники

Все запросы защищены сессией. В роли actor сервер использует сессию, а user_id ниже —
целевой участник, не заявленная идентичность вызывающего.

| Запрос | Payload запроса | Ответ |
| --- | --- | --- |
| CreateWorkspace | name, description: string | workspace: Workspace |
| ListWorkspaces | page: PageInput | page: Page<Workspace> |
| GetWorkspace | workspace_id: ID | workspace: Workspace |
| UpdateWorkspace | workspace_id: ID, name?: string, description?: string, expected_version: Version | workspace: Workspace |
| ListMembers | workspace_id: ID, page: PageInput | page: Page<Membership> |
| ListInvitations | page: PageInput | page: Page<Invitation> для текущего пользователя |
| InviteMember | workspace_id: ID, invitee_id: ID, role: admin/editor/viewer | invitation: Invitation |
| RespondToInvitation | invitation_id: ID, decision: accept/decline | invitation: Invitation |
| RevokeInvitation | invitation_id: ID | {} |
| ChangeRole | workspace_id: ID, user_id: ID, role: admin/editor/viewer | membership: Membership |
| RemoveMember | workspace_id: ID, user_id: ID | {} |
| LeaveWorkspace | workspace_id: ID | {} |
| TransferOwnership | workspace_id: ID, new_owner_id: ID, expected_version: Version | workspace: Workspace |
| DeleteWorkspace | workspace_id: ID, expected_version: Version, confirmed: true | {} |

Смена владельца — отдельная операция; ChangeRole не назначает owner. Сервер проверяет
границы owner/admin и запрещает invitation/leave/transfer для личного пространства.
Список приглашений доступен независимо от категории уведомлений. Выбор текущего
пространства клиент сохраняет локально: сообщения SelectWorkspace не требуется.
Для безверсионных membership-операций правила конкуренции OPEN; формат не объявляет
их идемпотентными. Повторное создание пространства/приглашения не имеет гарантии
идемпотентности только благодаря request_id.

## CLI → document-service: документы

Все запросы защищены сессией; доступ проверяется по актуальному членству.

| Запрос | Payload запроса | Ответ |
| --- | --- | --- |
| CreateDocument | workspace_id: ID, title: string, content: string | document: Document |
| ListDocuments | workspace_id?: ID, page: PageInput | page: Page<DocumentSummary> |
| SearchDocuments | workspace_id?: ID, query: string, page: PageInput | page: Page<DocumentSummary> |
| GetDocument | document_id: ID | document: Document |
| UpdateDocument | document_id: ID, title: string, content: string, expected_version: Version | document: Document, changed: boolean |
| DeleteDocument | document_id: ID, expected_version: Version, confirmed: true | {} |
| SubscribeDocument | document_id: ID | subscription: Subscription (enabled=true) |
| UnsubscribeDocument | document_id: ID | subscription: Subscription (enabled=false) |

DocumentsRemote.List с пустым Search отображается в ListDocuments, с непустым —
в SearchDocuments. При отсутствии workspace_id выборка охватывает только доступные
пространства. UpdateDocument передаёт полный заголовок/текст и проверяет ожидаемую
версию: changed=false означает подтверждённое отсутствие изменений без нового события.
Подписка — интерес к сообщениям, не право доступа; повтор того же состояния — no-op.

BeginEdit, SaveEdit, CompareLatest, RebaseDraft и DiscardDraft — внутренние операции.
Они используют GetDocument/UpdateDocument и локальный DraftStore; отдельных серверных
сообщений BeginEdit, RebaseDraft или LaunchEditor не требуется.

## Ошибки document-service → CLI

`Error = {code: string, message: string, field_errors?: FieldError[],
current_version?: Version, effect?: rejected/committed/unknown}`.
`FieldError = {field: string, code: string}`; только безопасные имена полей и коды.
message — безопасное пояснение, не машинный признак ошибки. Клиент использует code.
current_version выдаётся только при допустимом доступе к объекту.

| code | Значение для клиента |
| --- | --- |
| validation | Неверные поля/подтверждение/состояние операции; исправить ввод |
| unauthenticated | Сессия невалидна; очищать только использованный токен защищённого запроса, не более новый вход |
| denied | Операция запрещена; локальную сессию сохранить |
| not_found | Объект отсутствует или не раскрывается из-за доступа |
| conflict | Версия/уникальность/состояние конфликтуют; автоматической перезаписи нет |
| unavailable | Зависимость недоступна; для мутации обязательно учитывать effect |
| outcome_unknown | Сервер не может подтвердить исход всей операции; автоматического повтора нет |

effect уточняет исход мутации: rejected — подтверждённо нет изменений; committed —
предметная запись подтверждена, но последующий шаг не завершён; unknown — полный исход
неизвестен. Поле и точная модель частичных результатов — предложение, особенно для
DeleteSelf и обновления настроек. При отсутствии effect клиент не делает вывод о rollback.

Таймаут/обрыв без ответа — **локальная ошибка адаптера**, не полученное сообщение Error.
Если запись могла дойти до сервера, адаптер возвращает внутренний outcome_unknown
(также при отмене context); черновик сохраняется. Ошибки файлов, редактора и cleanup
остаются внутренними. Идемпотентность, retry-after, сопоставление HTTP/gRPC статусов
и восстановление неизвестного исхода OPEN.

### Пример: сохранение документа и конфликт

Пример не содержит session token: он передаётся защищёнными headers/metadata.

```json
{
  "schema_version": 1,
  "request_id": "req-save-1",
  "operation": "UpdateDocument",
  "payload": {
    "document_id": "doc-42",
    "title": "План проекта",
    "content": "# План\nНовый текст",
    "expected_version": 7
  }
}
```

```json
{
  "schema_version": 1,
  "request_id": "req-save-1",
  "operation": "UpdateDocument",
  "status": "success",
  "payload": {
    "document": {
      "id": "doc-42",
      "workspace_id": "ws-9",
      "title": "План проекта",
      "content": "# План\nНовый текст",
      "version": 8
    },
    "changed": true
  }
}
```

Альтернативный ответ при устаревшей версии:

```json
{
  "schema_version": 1,
  "request_id": "req-save-1",
  "operation": "UpdateDocument",
  "status": "error",
  "error": {
    "code": "conflict",
    "message": "Документ изменился",
    "current_version": 8,
    "effect": "rejected"
  }
}
```

## CLI ↔ notification-service: подключение

| Направление | Сообщение / действие | Данные |
| --- | --- | --- |
| CLI → HNS | WebSocket handshake over TLS | Тот же opaque session token; способ передачи/endpoint OPEN, не token в URL |
| HNS → CLI | Успешное установление соединения | Подтверждённая сессия; отдельный application Connected frame пока не требуется |
| HNS → CLI | Отказ handshake | unauthenticated либо unavailable; не обычное входящее уведомление |
| CLI / HNS → другая сторона | Закрытие соединения | Конкретные close codes/reason и heartbeat OPEN |

Получение токена HNS проверяет в Redis самостоятельно. При logout клиент закрывает
свой сокет. Скорость отзыва сессии/прав на уже открытом соединении не определена.
Повторное подключение аутентифицируется заново. Состояние подключения не равно
постоянной сессии и не означает доставку/прочтение сообщения.

## HNS → CLI: live-сигнал и персональное сообщение

Предлагаемый общий конверт **публичного сообщения**, а не Kafka-события:

| Поле | Тип | Назначение |
| --- | --- | --- |
| schema_version | integer | Версия публичного формата |
| message_id | ID | Идентичность сообщения; сохранение при повторе предлагается, дедуп/срок хранения OPEN |
| kind | refresh / notice | Обновить представление или показать персональное сообщение |
| type | string | Тип из каталога ниже; определяет payload |
| occurred_at | Timestamp | Время исходного изменения; не гарантия порядка доставки |
| delivery | live / backlog | Текущая доставка или накопленное offline-сообщение; не признак прочтения |
| source_event_id? | ID | Связь с исходным фактом, если есть; не ключ доставки или авторизация |
| workspace_id? | ID | Контекст пространства, если нужен |
| target? | TargetRef | {type: user/workspace/document/invitation, id: ID}; отсутствует для неоткрываемого факта |
| payload | object | Поля соответствующего типа ниже |

Адресат не берётся из произвольного client payload: он определён аутентифицированным
соединением и серверной маршрутизацией. Конверт не содержит session token, email,
credentials или полный текст документа. HNS не обязан раскрывать actor/target, если
это нарушает доступ. Набор типов ниже — проект, не утверждённая матрица получателей.

| type | kind | payload | Назначение / статус |
| --- | --- | --- | --- |
| DocumentChanged | refresh | document_id: ID, version: Version | Перечитать доступный документ/список; точный состав адресатов OPEN |
| WorkspaceChanged | refresh | workspace_id: ID, version?: Version | Обновить пространство/список; причины и получатели OPEN |
| ProfileChanged | notice | changed_fields: (username/email)[] | Владельцу при включённой users; сами значения email не передаются |
| WorkspaceInvitationReceived | notice | invitation_id: ID, workspace_id: ID, role: admin/editor/viewer | Адресату приглашения при включённой категории |
| WorkspaceInvitationResponded | notice | invitation_id: ID, decision: accepted/declined | Пригласившему согласно политике |
| WorkspaceRoleChanged | notice | workspace_id: ID, role: admin/editor/viewer | Затронутому участнику; передача owner отдельным типом |
| WorkspaceMemberRemoved | notice | workspace_id: ID | Минимальный факт бывшему участнику; без контента и доступа |
| WorkspaceOwnershipTransferred | notice | workspace_id: ID | Прежнему/новому владельцу; актуальное состояние читать через API |
| WorkspaceDeleted | notice | workspace_id: ID | Минимальный факт бывшим участникам; без target для открытия |
| DocumentUpdated | notice | document_id: ID, version: Version | Подписчикам с актуальным доступом; отличается от refresh |
| DocumentDeleted | notice | document_id: ID, workspace_id: ID | Минимальный факт eligible подписчикам; без target для открытия |

Отдельное персональное уведомление для UserCreated/UserDeleted, DocumentCreated,
подписки, переименования пространства или истечения приглашения не предполагается
исходной политикой. Это не запрещает отдельный refresh-сигнал при согласовании
матрицы live-обновлений. Комментарии/упоминания — будущая функциональность: сообщения
из старого client/messages.go не считаются согласованным публичным контрактом.

HNS отправляет live-сообщения во все активные подключения получателя. Offline-сообщения
сохраняются; накопленное получает одна подключившаяся сессия. Классы сохраняемых
сообщений, согласование live с backlog и гонки подключения OPEN. Поле delivery не
обещает ни отсутствие дубликатов, ни отсутствие потерь. Нельзя удалять pending только
по успешному socket write. Ack/replay/cursor, порядок, backpressure и совместимость
неизвестных типов требуют отдельного решения; клиент не выполняет действий по
неизвестному payload и не считает его обработанным подтверждением доставки.

### Пример: входящий сигнал обновления документа

```json
{
  "schema_version": 1,
  "message_id": "msg-5",
  "kind": "refresh",
  "type": "DocumentChanged",
  "occurred_at": "2026-11-09T12:00:00Z",
  "delivery": "live",
  "source_event_id": "evt-17",
  "workspace_id": "ws-9",
  "target": {"type": "document", "id": "doc-42"},
  "payload": {"document_id": "doc-42", "version": 8}
}
```

Клиент инициирует GetDocument/GetWorkspace при необходимости; сервер повторно проверяет
доступ. Открытый локальный черновик не перезаписывается этим сигналом.

### Пример: накопленное приглашение

```json
{
  "schema_version": 1,
  "message_id": "msg-6",
  "kind": "notice",
  "type": "WorkspaceInvitationReceived",
  "occurred_at": "2026-11-09T11:50:00Z",
  "delivery": "backlog",
  "source_event_id": "evt-18",
  "workspace_id": "ws-9",
  "target": {"type": "invitation", "id": "inv-3"},
  "payload": {"invitation_id": "inv-3", "workspace_id": "ws-9", "role": "editor"}
}
```

Наличие приглашения/его актуальный статус проверяются через ListInvitations;
принятие отправляет RespondToInvitation. Потеря/подавление сообщения не скрывает
приглашение из списка сервера.

## Сообщения, для которых пока описана только граница

| Направление | Кандидат | Что ещё согласовать |
| --- | --- | --- |
| CLI → HNS | Ack(message_id / batch / cursor) | Что подтверждается: получение, локальное сохранение или чтение; scope сессии и удаление pending |
| CLI → HNS | Resume(cursor) / запрос следующей backlog-порции | Нужен ли явный запрос, эксклюзивность одной сессии, cursor/lease и повторы |
| CLI ↔ HNS | ListInbox / UnreadCount / MarkRead / MarkAllRead | Нужна ли постоянная история; фильтры, read-state, boundary, видимость и счётчик |
| CLI → HDS | UpdateUserSettings(category, enabled, expected_version) | Предложено ADR-003; category=users/workspaces/documents, имя операции/схема и частичные исходы OPEN |
| HDS → CLI | UserSettings result | Версия сохранённого состояния и отдельное состояние распространения, а не обещание синхронного применения везде |

Redis UserSettingsChanged(user_id, version) — внутреннее HDS → HNS сообщение,
**не** входящее WebSocket-сообщение CLI. Если UI требуется подтверждение применения
на всех соединениях, его отдельный внешний контракт ещё нужно определить.

## Связь со сценариями

| Сценарий | Внешние сообщения |
| --- | --- |
| [auth](../../scenarios/auth.puml) | Authenticate, Logout, защищённые запросы и handshake |
| [users](../../scenarios/users.puml) | Register, профили, подтверждение email, удаление |
| [workspaces](../../scenarios/workspaces.puml) | CRUD пространств, приглашения, membership и владение |
| [documents](../../scenarios/documents.puml) | CRUD/поиск документов и подписки; локальный редактор вне сети |
| [events_dispatch](../../scenarios/events_dispatch.puml) | Результат команды отдельно от последующего live/notice |
| [notifications_inbox](../../scenarios/notifications_inbox.puml) | WebSocket, backlog, target read и предложенные настройки |
