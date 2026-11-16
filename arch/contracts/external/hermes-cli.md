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
| IdentityConfirmation | method: string, proof: object; подтверждение личности для опасной операции, формат OPEN |

Это DTO сообщения, а не сериализация внутренних структур Go.
Credentials.Password, SessionKey, ServerIdentity, Draft, RecoveryPath, локальные пути,
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
| Register | username: string, display_name: string, email: string, password: string | profile: Profile, personal_workspace: Workspace | Нет |
| Authenticate | username: string, password: string | user_id: ID, token: string, expires_at: Timestamp | Нет |
| Logout | {} | {} после подтверждённого отзыва/отсутствия сессии | Отзываемый токен; отсутствие/истечение допустимо |
| GetSelf | {} | profile: Profile | Да |
| GetPublicProfile | user_id: ID целевого пользователя | profile: PublicProfile | Да |
| UpdateSelf | username?: string, display_name?: string, expected_version: Version | profile: Profile | Да |
| RequestEmailChange | new_email: string, expected_version: Version | confirmation_required: true | Да |
| ConfirmEmail | confirmation_token: string | profile: Profile | Одноразовый confirmation token; повторный session token не предполагается |
| GetDeletionBlockers | {} | workspaces: Workspace[] | Да |
| DeleteSelf | expected_version: Version, confirmation: IdentityConfirmation | {} только после подтверждённого удаления и отзыва сессий | Да |

Регистрация атомарно создаёт пользователя, личное пространство и owner membership,
но не выполняет вход. Логин — username, сравниваемый без учёта регистра; пароль
проверяется точно. Вход по email не предусмотрен. Способ выбран в
[ADR-004](../../decisions/004-password-authentication.md); исходная email verification OPEN. GetPublicProfile ограничен общей активной membership.
RequestEmailChange сохраняет запрос, а не меняет email. ConfirmEmail меняет email
только после проверки одноразового секрета, срока и уникальности; секрет не логируется.
Предварительные deletion blockers не отменяют повторных проверок при DeleteSelf.
Частичный/неизвестный результат PostgreSQL + Redis не выдаётся за success.

## CLI → document-service: пространства и участники

Все запросы защищены сессией. В роли actor сервер использует сессию, а user_id ниже —
целевой участник, не заявленная идентичность вызывающего.

| Запрос | Payload запроса | Ответ |
| --- | --- | --- |
| CreateWorkspace | name: string, description: string | workspace: Workspace |
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

## Правила обработки запросов document-service

Этот раздел уточняет проект каталога выше; он не означает, что сервер или сетевой
адаптер уже реализован. Все поля payload обязательны, если не помечены `?`.
Пустой результат операции передаётся как `{}`, пустой список — как `items: []`.
`null` не заменяет отсутствие optional-поля. ID, токены и cursor непрозрачны для CLI.
Сервер задаёт идентификаторы, версии и сроки действия; клиент их не генерирует.
Пустой текст документа и пустое описание допустимы; заголовок документа и имя
пространства не могут быть пустыми. Точные лимиты строк и правила нормализации OPEN.

Для UpdateSelf и UpdateWorkspace требуется хотя бы одно изменяемое поле.
Переданное пустое описание очищает описание; пропущенное оставляет его прежним.
UpdateDocument — полная замена title/content, а не patch. workspace_id документа
через него изменить нельзя. Первую версию создаваемых объектов предлагается считать 1.

Проверка сессии и актуального доступа предшествует раскрытию состояния объекта.
Проверка expected_version и запись выполняются атомарно: устаревшая версия возвращает
conflict даже при совпадении присланного текста с текущим. Успешное изменение увеличивает
версию объекта; no-op сохраняет её. UpdateDocument возвращает changed=false только
после успешных проверок доступа и версии. Подтверждение удаления не заменяет эти проверки.
expected_version относится к Profile для UpdateSelf/RequestEmailChange/DeleteSelf,
Workspace для UpdateWorkspace/TransferOwnership/DeleteWorkspace и Document для
UpdateDocument/DeleteDocument. Запрос смены email не считается подтверждённой сменой
профиля; момент увеличения версии для pending-запроса ещё нужно согласовать.

### Доступ и предметные отказы

Таблица дополняет поля сообщений. Общие ошибки validation, unauthenticated,
unavailable и outcome_unknown определены ниже и здесь не повторяются.
Выбор denied или скрывающего существование not_found должен быть единым для
конкретного класса объектов; точная политика раскрытия OPEN.

| Операции | Проверки и предметные отказы |
| --- | --- |
| Register | Уникальность username/email без учёта регистра; конфликт уникальности — conflict. Личное пространство и membership создаются в одной транзакции с профилем |
| Authenticate | Проверка username/password; неверные учётные данные — unauthenticated без раскрытия существования пользователя. Одного username недостаточно |
| Logout | Отзывается предъявленный токен. Уже отсутствующая или истёкшая сессия даёт success; недоступность Redis не выдаётся за подтверждённый отзыв |
| GetSelf, UpdateSelf, RequestEmailChange | Только собственный профиль. UpdateSelf и RequestEmailChange проверяют версию; занятые username/email — conflict |
| GetPublicProfile | Только публичные поля пользователя с общим активным членством; email и credentials не выдаются |
| ConfirmEmail | Проверить срок, одноразовость и назначение секрета, повторно проверить уникальность email. Недействительный секрет — validation; занятый email — conflict. Поведение повтора после успеха OPEN |
| GetDeletionBlockers, DeleteSelf | Blockers — только общие пространства, которыми владеет пользователь. Наличие blockers при DeleteSelf — conflict; подтверждение личности и версия проверяются заново |
| CreateWorkspace, ListWorkspaces, GetWorkspace | Создаётся shared-пространство с actor как owner; список содержит только доступные пространства, включая личное |
| UpdateWorkspace | Owner/admin; личное пространство меняет только владелец; устаревшая версия — conflict |
| ListMembers | Только участник соответствующего пространства |
| ListInvitations | Только приглашения, адресованные текущему пользователю; выдача pending/истории и порядок пока OPEN |
| InviteMember | Только shared. Owner приглашает admin/editor/viewer, admin — editor/viewer; существующее членство или pending-приглашение — conflict |
| RespondToInvitation | Только адресат, pending и неистёкшее приглашение в активное пространство. Смена состояния и создание membership при accept атомарны; устаревшее состояние — conflict |
| RevokeInvitation | Проверить актуальные права управления и pending-состояние; accept/revoke/expiry сериализуются. Политика отзыва приглашения на роль admin требует уточнения |
| ChangeRole, RemoveMember | Только shared; owner управляет admin/editor/viewer, admin — editor/viewer. Owner защищён от обычной смены роли и исключения |
| LeaveWorkspace | Только shared, текущий участник, кроме owner. Владелец сначала передаёт владение или удаляет пространство |
| TransferOwnership | Только текущий owner shared-пространства; новый владелец — активный участник. Проверить версию; прежний owner становится admin в той же транзакции |
| DeleteWorkspace | Только owner shared-пространства, актуальная версия и confirmed=true. Атомарно закрыть доступ к документам, членствам и приглашениям |
| ListDocuments, SearchDocuments, GetDocument | Только актуальные читатели. Без workspace_id поиск/список охватывают доступные пространства; с явным workspace_id сначала проверяется доступ к нему |
| CreateDocument, UpdateDocument, DeleteDocument | Owner/admin/editor соответствующего пространства. Для обновления/удаления проверить версию; удаление требует confirmed=true |
| SubscribeDocument, UnsubscribeDocument | Любой актуальный читатель; меняется подписка только текущего пользователя. Повтор уже установленного состояния — success без нового изменения |

Ни одна мутация не принимает actor_id, created_by, updated_by или собственную роль
вызывающего. user_id/invitee_id/new_owner_id — идентификаторы целей операции.
Register и Authenticate передают секреты только по защищённому соединению; их payload
и успешный Authenticate с token исключаются из логирования. Действительность
confirmation_token не заменяет session token в других операциях.

### Списки и поиск

Ответ списка всегда имеет вид `payload: {page: {items: [...], next_cursor?: Cursor}}`.
Для первого запроса CLI передаёт `page: {}` либо page с limit; следующий запрос
использует полученный cursor, ту же operation и те же фильтры. Cursor не даёт прав:
сервер заново ограничивает выдачу текущим доступом. Невалидный cursor — validation;
срок жизни cursor и поведение при изменении фильтров OPEN.
Сортировка, семантика поиска (по заголовку/тексту, регистр, язык), обработка пустого query,
максимум limit и согласованность страниц пока не закреплены контрактом.

### Успех, повтор и неизвестный исход

Успешный ответ на изменение подтверждает предметный результат, но не получение
уведомления другими клиентами и не завершение Kafka/WebSocket-доставки.
DeleteSelf имеет дополнительное условие подтверждённого отзыва сессий, указанное выше.
Успешное удаление возвращает `{}`, а не удалённый документ или список его получателей.

request_id служит корреляции и не предотвращает повторное выполнение. Автоматический
повтор создания, принятия приглашения, передачи владения и удаления пока не разрешён
этим контрактом. Повторяемое чтение также не обещает тот же снимок данных.
После неизвестного исхода UpdateDocument CLI сохраняет черновик и может запросить
GetDocument для сравнения: совпадение текста само по себе не доказывает, кто выполнил
изменение. После неизвестного исхода DeleteSelf/Logout нельзя заявлять подтверждённый
серверный отзыв. Протокол идемпотентности и отдельный запрос статуса операции OPEN.

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

### Пример: вход

Запрос Authenticate передаёт username и password. Пароль CLI получает скрытым вводом,
не аргументом командной строки. Пример содержит вымышленные данные:

```json
{
  "schema_version": 1,
  "request_id": "req-auth-1",
  "operation": "Authenticate",
  "payload": {
    "username": "microice",
    "password": "example-only-not-a-real-password"
  }
}
```

Register передаёт `payload: {username, display_name, email, password}`; пароль
не возвращается в Profile. Сервер хранит только солёный хеш пароля.
Текущая CLI-команда `auth <username>` остаётся заглушкой.
После успешной проверки сервер отвечает (значение token ниже вымышленное):

```json
{
  "schema_version": 1,
  "request_id": "req-auth-1",
  "operation": "Authenticate",
  "status": "success",
  "payload": {
    "user_id": "user-12",
    "token": "example-only-not-a-real-token",
    "expires_at": "2026-11-17T12:00:00Z"
  }
}
```

CLI сохраняет сессию локально и передаёт токен в защищённых headers/metadata
последующих запросов. Токен не включается в payload GetSelf или GetDocument.

### Пример: список документов

```json
{
  "schema_version": 1,
  "request_id": "req-list-1",
  "operation": "ListDocuments",
  "payload": {
    "workspace_id": "ws-9",
    "page": {"limit": 20}
  }
}
```

```json
{
  "schema_version": 1,
  "request_id": "req-list-1",
  "operation": "ListDocuments",
  "status": "success",
  "payload": {
    "page": {
      "items": [
        {
          "id": "doc-42",
          "workspace_id": "ws-9",
          "title": "План проекта",
          "version": 7
        }
      ],
      "next_cursor": "example-opaque-cursor"
    }
  }
}
```

Лимит 20 — пример, не установленный серверный предел. Для продолжения передаётся
`page: {limit: 20, cursor: "example-opaque-cursor"}` с тем же workspace_id.
GetDocument с `payload: {document_id: "doc-42"}` вернёт `payload: {document: ...}`:
те же поля DocumentSummary и полный content. В списке полного content нет.

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
