# Lesestatus im geteilten Posteingang

Stand: 30.09.2026 · Anlass: Issue #3673

## Fragestellung

Wie lösen Produkte mit einem geteilten Posteingang „gelesen“, „ungelesen“ und „erledigt“ für ein Team? Pro Produkt geprüft:

1. Ist „gelesen“ persönlich, team-weit oder beides?
2. Gibt es „erledigt / geschlossen / archiviert“ getrennt von „gelesen“?
3. Gibt es Zuständigkeit (Person, Team, Rolle), und steuert sie Zähler oder Badges?
4. Kann eine Person Zähler oder Benachrichtigungen für sich abschalten?
5. Was verhindert, dass eine Person etwas vor der zuständigen Person „versteckt“?
6. Was sieht der Absender (Kundschaft, Eltern) als Lesebestätigung, und wessen Lesen löst sie aus?

Belegt sind nur Aussagen aus offiziellen Hilfecentern und Produktdokumentationen. Was dort nicht steht, ist als **nicht belegt** markiert. Ableitungen sind als solche gekennzeichnet.

Ausgangslage moto: Lesestand pro Konto (Cursor). Eine Antwort von irgendwem erledigt die Unterhaltung für das ganze Team. Jede Person kann „für das Team als ungelesen markieren“ (#3654) und „alle als gelesen markieren“ für sich selbst (#3663). Eltern sehen „Von der OGS gelesen“, abgeleitet aus den Cursorn der Mitarbeitenden.

## Vergleich

| Produkt | 1. Gelesen | 2. Erledigt getrennt | 3. Zuständigkeit steuert Zähler | 4. Eigene Zähler / Benachrichtigungen | 5. Schutz vor Verstecken | 6. Lesebestätigung beim Absender |
|---|---|---|---|---|---|---|
| Front | persönlich | ja (Archivieren, team-weit im Shared Inbox) | ja („Assigned to me“, Tabs Unassigned/Assigned) | ja (Zählerart, Benachrichtigungsregeln) | Archivieren durch Nicht-Zuständige lässt die Unterhaltung bei der zuständigen Person offen | nicht belegt |
| Help Scout | kein Lesestatus für Unterhaltungen belegt | ja (Active / Pending / Closed) | ja (Mine, Unassigned, Assigned; Zähler = Active) | ja (pro Person und pro Postfach) | Kundenantwort setzt auf Active zurück; Zuweisung protokolliert; Kollisionserkennung | nicht belegt |
| Zendesk | Benachrichtigungsliste pro Person | ja (New, Open, Pending, Solved, Closed) | ja (Views „deine“, „deine Gruppen“, „nicht zugewiesen“) | ja („alle als gelesen“ in der eigenen Liste) | nicht belegt | Richtung Kundschaft nicht belegt; Agenten sehen „Read“ der Kundschaft |
| Intercom | persönlich nur für Erwähnungen belegt | ja (Open / Closed / Snoozed) | ja (Unassigned-Zähler; Benachrichtigung auch für Team-Zuweisung) | ja (persönliche Benachrichtigungseinstellungen) | Schließen ändert Zuweisung nicht | „Seen“ erst, wenn irgendwer aus dem Team zu antworten beginnt, nicht beim bloßen Öffnen |
| Google Groups Collaborative Inbox | persönlich (über eigene Gmail-Kopie) | ja (Complete, No action needed, Duplicate) | Filter „Assigned to me“; Mail an zugewiesene Person | Abo-Einstellung pro Mitglied | Erledigen nur mit Berechtigung „moderate metadata“ | entfällt (E-Mail) |
| Outlook Shared Mailbox | team-weit (ein Postfach) | kein eigener Status; Kategorien, Ordner | nein | nicht belegt | keiner; Lesen versteckt für alle | entfällt |
| Exchange Public Folder | umschaltbar: pro Person möglich | nicht belegt | nein | nicht belegt | nicht belegt | entfällt |
| Sdui | persönlich (eigene Markierung, eigener Punkt) | Archivieren nur für sich | nicht belegt | ja (Chat stumm schalten, Ruhezeiten) | „Anklopfen“ geht nur an Gruppen-Admin | Verfasser sieht Leseliste mit Namen |
| SchoolFox / KidsFox | nicht belegt | „Erledigt“-Ablage (Geltungsbereich nicht belegt) | nicht belegt | nicht belegt | Kolleginnen und Kollegen nur als „Mitlesende“, für Absender sichtbar | Lesebestätigung der Eltern ist eine bewusste Aktion; Gegenrichtung nicht belegt |
| ParentSquare | nicht belegt | nicht belegt | nicht belegt | Sprechzeiten unterdrücken Push | nicht belegt | nur eingeschränkt belegt: Read-Status nur für Mitarbeitende, Eltern sehen keine |
| Brightwheel | „als ungelesen markieren“ vorhanden, Geltungsbereich nicht belegt | nicht belegt | Sichtbarkeit: alle Mitarbeitenden oder nur Admins | nicht belegt | Admin-Unterhaltung für alle Admins sichtbar | nicht belegt |
| Famly | vermutlich persönlich (Ableitung) | Archivieren (Eltern-Seite belegt) | Raum-Posteingang nur für Mitarbeitende, die den Raum favorisieren | ja (Favoriten wählen, „alle als gelesen“ pro Posteingang) | Nachricht an Leitung kann nicht abgeschaltet werden | „Seen by“ pro Einrichtung abschaltbar, auch nur für Eltern |
| Procare | nicht belegt | nicht belegt | Kanal steuert Empfänger: Office Chat nur Admins, Classroom Chat Gruppe | nicht belegt | nicht belegt | nicht belegt |
| Stay Informed | nicht belegt für Messenger | nicht belegt | Einzelchat nur mit Verwaltungsberechtigten | ja (Messenger abschalten, Verfügbarkeitszeiten) | nicht belegt | nicht belegt |

## Geteilte Postfächer

### Front

- **Gelesen ist persönlich.** „Marking a conversation as read or unread applies only to you … your teammates will still see the conversation as unread.“ Eine Team-Aktion „für alle ungelesen“ ist nicht dokumentiert. Front empfiehlt ausdrücklich Archivieren statt Lesen als Arbeitsablauf. ([Read vs. unread status](https://help.front.com/en/articles/2164))
- **Erledigt ist getrennt und team-weit.** Archivieren im Shared Inbox wirkt für alle, die dieses Postfach ansehen. Neue Nachrichten holen die Unterhaltung zurück. ([Conversation status](https://help.front.com/en/articles/2134))
- **Zuständigkeit schützt.** „If a teammate who is not the assignee changes a conversation's status from a shared inbox, then the conversation will stay open in the assignee's Assigned to me section.“ ([Conversation status](https://help.front.com/en/articles/2134))
- **Zähler pro Person.** Für den eigenen Bereich wählt jede Person „Unread“ oder „Open“ als Zählerart. Für Shared Inboxes wählt sie Tabs (Unassigned plus Assigned, oder Open); die Zähler folgen dieser Wahl. ([Individual inbox counters](https://help.front.com/en/articles/2234), [Inbox tabs](https://help.front.com/en/articles/2159))
- **Benachrichtigungen pro Person.** Modi „All“, „Smart“, „No notifications“ plus eigene Regeln. „Smart“ meldet Zuweisung, Erwähnung und neue Aktivität in zugewiesenen Unterhaltungen, nicht jede neue Nachricht im Shared Inbox. ([Notification settings](https://help.front.com/en/articles/2174))
- **Transparenz.** Das Teilnehmermenü zeigt pro Person, ob sie die letzte Nachricht gelesen hat („Unread“). ([Participants menu](https://help.front.com/en/articles/2258))
- Lesebestätigung für Kundschaft: nicht belegt.

### Help Scout

- **Lesestatus pro Unterhaltung:** nicht belegt. Die Doku arbeitet mit Status: Active (wartet auf Handlung), Pending, Closed. Eine Kundenantwort setzt Pending oder Closed wieder auf Active. ([Conversation icons and colors](https://docs.helpscout.com/article/11-understand-conversation-icons-and-colors))
- **Zähler folgen Status und Zuständigkeit.** Ordner Unassigned, Mine, Assigned; der Zähler zeigt jeweils nur Active-Unterhaltungen. ([Default folder views](https://docs.helpscout.com/article/1429-about-default-folder-views-in-help-scout))
- **Zuweisung.** An Personen oder (Plus/Pro) Teams. Alle mit Postfachzugriff sehen und ändern Zuweisungen; ein Protokolleintrag zeigt, wer wem zugewiesen hat. ([Assign conversations](https://docs.helpscout.com/article/842-assign-conversations))
- **Benachrichtigungen pro Person und pro Postfach** einstellbar; Erwähnungen und gefolgte Unterhaltungen erscheinen immer in der Glocke. ([Manage notifications](https://docs.helpscout.com/article/696-manage-help-scout-notifications))
- **Kollisionserkennung** zeigt, wer gerade liest oder antwortet, und hält eine Antwort an, wenn inzwischen jemand anderes geantwortet hat. ([Collision detection](https://docs.helpscout.com/article/99-prevent-duplicate-replies-with-collision-detection))
- Lesebestätigung für Kundschaft: nicht belegt.

### Zendesk

- **Persönliche Benachrichtigungsliste** mit Ungelesen-Zähler pro Agent: neue Kundenantwort auf eigene oder gefolgte Tickets, Zuweisung, Erwähnung. Es gibt „alle als gelesen markieren“ für die eigene Liste. ([Notifications list](https://support.zendesk.com/hc/en-us/articles/4408829025690-Using-the-notifications-list-to-manage-conversations))
- **Erledigt ist ein Ticketstatus** (New, Open, Pending, On-hold, Solved, Closed), unabhängig vom Lesen. ([Ticket lifecycle](https://support.zendesk.com/hc/en-us/articles/8263915942938-About-the-ticket-lifecycle-and-ticket-statuses))
- **Zuständigkeit über Person und Gruppe.** Standard-Views wie „Your unsolved tickets“, „Unsolved tickets in your groups“, „Unassigned tickets“. ([Views](https://support.zendesk.com/hc/en-us/articles/4408829483930-Accessing-your-views-of-tickets), [Groups](https://support.zendesk.com/hc/en-us/articles/4408839035546-Using-groups))
- **Lesebestätigung:** Im Messaging sieht der Agent, ob die Kundschaft seine Nachricht gelesen hat. Ob die Kundschaft sieht, dass ein Agent gelesen hat: nicht belegt. ([Messaging in Agent Workspace](https://support.zendesk.com/hc/en-us/articles/4408843683226-Receiving-and-sending-messages-in-the-Zendesk-Agent-Workspace))

### Intercom

- **Ungelesen persönlich** ist nur für Erwähnungen belegt: „This will only mark the mention as unread for you … You cannot mark a customer reply as unread.“ ([Loop teammates in](https://www.intercom.com/help/en/articles/6525765-loop-teammates-or-teams-into-conversations))
- **Erledigt getrennt:** Open, Closed, Snoozed. „When a conversation is closed by a teammate or automation, the assigned teammate and team remain unchanged.“ ([Assign conversations](https://www.intercom.com/help/en/articles/6561699-assign-conversations-to-teammates-and-teams-in-the-next-gen-inbox), [The Inbox explained](https://www.intercom.com/help/en/articles/6258745-the-inbox-explained))
- **Zuständigkeit.** Der Unassigned-Zähler zeigt offene Unterhaltungen ohne Person oder Team. Wer für eigene Zuweisungen Benachrichtigungen aktiv hat, wird auch bei Zuweisung an ein eigenes Team benachrichtigt. Einstellungen pro Person unter Personal > Notifications. ([Organize team inboxes](https://www.intercom.com/help/en/articles/197-organize-team-inboxes), [How teammates get notifications](https://www.intercom.com/help/en/articles/187-how-teammates-get-notifications))
- **Lesebestätigung für Kundschaft, besonders relevant für moto:** Eine neue Nachricht bleibt „Not yet seen“, bis jemand aus dem Team zu tippen beginnt. „This means your team can view, assign or add notes to new conversations without impacting your customers' expectations.“ Nach der ersten Antwort gilt eine Folgenachricht als „Seen“, sobald irgendwer in das Antwortfeld klickt. Abschalten lässt sich das nicht. Ausnahme WhatsApp: dort reicht Öffnen. ([Real-time messaging explained](https://www.intercom.com/help/en/articles/258-real-time-messaging-explained))

### Google Groups Collaborative Inbox

- **Erledigt getrennt und rechtegebunden.** „Mark as complete“ braucht die Berechtigung „Who can moderate metadata“; „No action needed“ und „Duplicate“ brauchen „Who can moderate content“. Zuweisen, Übernehmen, Freigeben; zugewiesene Personen bekommen eine E-Mail; Filter „Assigned to me“, „Not assigned“, „unresolved“. ([Take & assign conversations](https://support.google.com/groups/answer/2467048?hl=en), [Collaborative Inbox](https://support.google.com/a/users/answer/167430?hl=en))
- **Gelesen:** Einen Ungelesen-Zähler gibt es laut Hilfe nur über einen Filter im eigenen Mailprogramm mit Abo „Each email“, also über die eigene Kopie jedes Mitglieds. Daraus folgt (Ableitung): Lesen ist persönlich und von „erledigt“ getrennt. Ein gemeinsamer Lesestatus in der Groups-Oberfläche ist nicht belegt. ([View unread messages](https://support.google.com/groups/answer/9792691?hl=en))

### Outlook / Exchange Shared Mailbox

- **Gelesen ist team-weit**, weil alle dasselbe Postfach mit Full Access bearbeiten. Die offizielle Microsoft-Doku sagt das nicht ausdrücklich; belegt ist es durch eine Moderatorenantwort auf Microsoft Q&A: „once a user reads the message or … marks as read, the message will appear as read for all users“. Als Umweg werden Kategorien genannt. ([Microsoft Q&A](https://learn.microsoft.com/en-us/answers/questions/4660699/shared-mailbox-individual-mail-read-unread-option), [Shared mailboxes](https://learn.microsoft.com/en-us/exchange/collaboration-exo/shared-mailboxes))
- **Public Folders** können den Lesestatus pro Person führen: „The PerUserReadStateEnabled parameter specifies whether to maintain read and unread data on a per-user basis.“ ([Set-PublicFolder](https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-publicfolder))
- Eigenen Status „erledigt“, Zuständigkeit oder persönliche Zähler hat das Shared Mailbox nicht (Ableitung aus fehlender Doku; nicht belegt). Das ist das Gegenbeispiel: Wer liest, versteckt die Nachricht für alle.

## Schul- und Kita-Kommunikation

### Sdui

- **Ungelesen markieren ist persönlich** und unterscheidet selbst gesetzte Markierung (Punkt) von echten neuen Nachrichten (Zahl). ([Chat als ungelesen markieren](https://support.sdui.de/de_DE/96059-chat/wie-kann-ich-einen-chat-als-ungelesen-markieren))
- **Archivieren nur für sich**, kommt bei neuer Nachricht zurück. ([Chat archivieren](https://support.sdui.de/de_DE/96059-chat/wie-kann-ich-einen-chat-archivieren))
- **Benachrichtigungen pro Chat abschaltbar**; danach nur noch Hinweis beim Öffnen der App. Dazu Ruhezeiten. ([Chats stumm schalten](https://support.sdui.de/de_DE/96059-chat/wie-kann-ich-bestimmte-chats-stumm-schalten), [Chat-Übersicht](https://support.sdui.de/de_DE/96066-chat))
- **Lesebestätigung mit Namen:** „Der Verfasser einer Nachricht kann sehen, wer diese bereits gelesen hat“ (Leseliste). Der Artikel steht im Bereich für Lehrkräfte; ob Eltern die Leseliste für eigene Nachrichten ebenso sehen: nicht belegt. ([Wer hat gelesen](https://support.sdui.de/de_DE/96059-chat/kann-ich-sehen-wer-meine-nachricht-schon-gelesen-hat))
- **Zuständigkeit:** Bei One-Way-Gruppen „klopfen“ Eltern an; die Nachricht erhält nur der Gruppen-Admin. ([Anklopfen](https://support.sdui.de/de_DE/96059-chat/wie-k%C3%B6nnen-mich-eltern-und-sch%C3%BClerinnen-erreichen-wenn-der-klassen-chat-auf-one-way-gestellt-ist))
- Ein geteilter Team-Posteingang für Elternnachrichten: nicht belegt. Sdui arbeitet mit Einzel- und Gruppenchats.

### SchoolFox / KidsFox (FoxEducation)

- **Lesebestätigung ist eine Aktion der Eltern** (Bestätigen, ggf. FoxSign), Richtung Schule zu Eltern. ([FoxSign](https://support.foxeducation.com/en/users/foxsign))
- **Sichtbarkeit im Team:** Lehrkräfte können Kolleginnen und Kollegen derselben Klasse als „readers“ hinzufügen; diese sind für die Absender sichtbar. ([Who can view my messages?](https://support.foxeducation.com/en/users/insight_messaging))
- **Abwesenheitsmeldungen** werden wie Nachrichten von den Empfängern bestätigt; sichtbar für FoxAdmins und alle zugeordneten Lehrkräfte. ([Absences](https://support.foxeducation.com/en/users/absences))
- **„Done“:** Nachrichten lassen sich von „Open Messages“ nach „Done“ verschieben. Ob persönlich oder geteilt: nicht belegt. ([Move a message to Done](https://support.foxeducation.com/en/users/organize-messages))
- Ob Eltern sehen, dass die Schule ihre Nachricht gelesen hat: nicht belegt.

### ParentSquare

- **Nur eingeschränkt belegt.** Ein Hilfeartikel „Manage Direct Messages“ in einer Zendesk-Instanz mit dem Titel „ParentSquare Sandbox“ war nur als Suchauszug lesbar (Seite: HTTP 403). Laut Auszug sehen nur Mitarbeitende „Sent“ und „Read“ bei Direktnachrichten; „Parents, students, and guests cannot view read receipts“; in Gruppenunterhaltungen gibt es keine. ([Manage Direct Messages](https://parentsquare1736363850.zendesk.com/hc/en-us/articles/41702676848013-Manage-Direct-Messages))
- Sprechzeiten, die Push-Benachrichtigungen außerhalb der Zeiten unterdrücken, sind nur über Seiten von Schulbezirken belegt, nicht über ParentSquare selbst.
- Geteilter Posteingang, Zuweisung, erledigt: nicht belegt.

### Brightwheel

- **Als gelesen / ungelesen markieren**, einzeln und in Masse, für alle Nutzerarten. Ob die Markierung nur für die markierende Person oder für alle Mitarbeitenden gilt: nicht belegt. ([Mark messages as read & unread](https://help.mybrightwheel.com/en/articles/9904214-mark-messages-as-read-unread))
- **Sichtbarkeit als Zuständigkeit:** Admins können Unterhaltungen mit einer Familie für alle Mitarbeitenden oder nur für Admins sichtbar machen. Eine Admin-Unterhaltung ist „not a direct message between a specific Admin/Manager and Parent“, sondern für alle Admins sichtbar. ([Admin-only messages](https://help.mybrightwheel.com/en/articles/8436776-send-messages-to-families-only-visible-to-admin))
- Lesebestätigung für Eltern: in der offiziellen Hilfe nicht belegt.

### Famly

- **Raum-Posteingang als Team-Inbox mit Opt-in.** Nachrichten an einen Raum sehen alle Mitarbeitenden, die den Raum als Favorit markiert haben. „All rooms“ bekommen nur Mitarbeitende mit der Berechtigung „Send private messages to parents“. Pro Posteingang gibt es „Mark all as read“. ([Your Messaging Inbox](https://help.famly.co/en/articles/6240469-your-messaging-inbox))
- **Eltern wählen den Empfänger:** Raum (alle Mitarbeitenden dort), einzelne Person oder Leitung. „The option to message Management is always available. It cannot be turned off.“ ([Your Messaging Inbox](https://help.famly.co/en/articles/6240469-your-messaging-inbox), [Parents: Send a Private Message](https://help.famly.co/en/articles/4912443-parents-send-a-private-message))
- **„Seen by“ ist pro Einrichtung einstellbar**, mit getrennten Schaltern: ganz aus, nur für Eltern aus, oder Sicht der Mitarbeitenden auf das Lesen der Leitung. Begründung: Erwartungen der Eltern steuern, besonders außerhalb der Arbeitszeit. Ausgelöst wird es, wenn „a member of staff has read their message“. ([Messaging Settings](https://help.famly.co/en/articles/5124016-messaging-settings-for-communicating-with-parents))
- Ob „gelesen“ pro Person geführt wird, steht nicht ausdrücklich da; „Mark all as read in each of your inboxes“ spricht dafür (Ableitung). Status „erledigt“ für Mitarbeitende: nicht belegt.

### Procare (Procare Online)

- **Zuständigkeit über Kanäle:** „Office Chat“ nur für Admins (z. B. Abrechnung), „Classroom Chat“ für die Gruppe. Office Chat erhalten „Staff with an ‘Admin’ role with Full Access to Parent / Staff Messaging“. Ungelesenes zeigt ein rosa Punkt pro Kanal. ([Parent Messaging via the Web](https://www.procaresupport.com/procare-online/docs/message-parents-website), [Message Parents, Mobile App](https://www.procaresupport.com/procare-online/docs/message-parents-app))
- Lesestatus pro Person oder Team, erledigt, Lesebestätigung für Eltern: nicht belegt.

### Stay Informed

- **Einzelchat nur mit Verwaltungsberechtigten.** Diese können Verfügbarkeitszeiten setzen; Eltern sehen dann einen Hinweis, und die Mitarbeitenden werden erst benachrichtigt, wenn sie wieder verfügbar sind. ([Allgemeine Informationen zum Messenger](https://stayinformed.zammad.com/help/de-de/94-chatfunktion/104-allgemeine-informationen-zum-messenger))
- **Messenger pro Konto abschaltbar** (Gruppenchats, Einzelchat). ([Chat aktivieren/deaktivieren](https://stayinformed.zammad.com/help/de-de/108-optionen/137-aktivieren-deaktivieren-des-chats))
- Lesestatus im Team, erledigt, Lesebestätigung im Messenger: nicht belegt.

## Muster

1. **Gelesen ist fast immer persönlich.** Front sagt es ausdrücklich, Sdui und Google Groups ergeben es, Zendesk führt eine persönliche Liste. Der einzige belegte team-weite Lesestatus ist das Outlook Shared Mailbox, und genau dort ist „einer liest, alle verlieren den Hinweis“ das bekannte Problem. Exchange bietet für Public Folders deshalb einen Schalter für den Lesestatus pro Person.
2. **Die Team-Wahrheit ist „erledigt“, nicht „gelesen“.** Front (Archivieren), Help Scout (Active/Closed), Zendesk (Solved), Intercom (Closed), Google (Complete) trennen den Arbeitsstand vom Lesen. Die Team-Zähler zählen offene oder aktive Unterhaltungen, nicht ungelesene.
3. **Zuständigkeit steuert, wer die Zahl sieht.** Unassigned, Mine, Assigned (Help Scout, Front, Intercom, Zendesk) oder Opt-in über Räume und Kanäle (Famly Favoriten, Procare Office Chat, Stay Informed Verwaltungsberechtigte). Wer nicht zuständig ist, sieht die Zahl gar nicht erst oder nur in einer Sammelansicht.
4. **Zähler und Benachrichtigungen sind persönliche Einstellungen.** Front (Zählerart, Tabs, Regeln), Help Scout (pro Postfach), Intercom, Sdui (stumm, Ruhezeiten), Stay Informed (Verfügbarkeit, Messenger aus). Eine Schul-weite Pflichteinstellung für Zähler ist in keinem Produkt belegt.
5. **Schutz vor Verstecken läuft über Zuständigkeit, Status-Rückfall und Rechte, nicht über gesperrtes Lesen.** Front lässt Archivieren durch Nicht-Zuständige bei der zuständigen Person offen. Help Scout und Front holen eine Unterhaltung bei neuer Kundennachricht zurück. Google bindet „erledigt“ an eine Berechtigung. Famly lässt den Kanal zur Leitung nicht abschalten. Help Scout protokolliert Zuweisungen.
6. **Eine Team-Aktion „für alle als ungelesen markieren“ ist in keinem geprüften Produkt belegt.** Wo es „ungelesen markieren“ gibt, gilt es für die eigene Person (Front, Intercom, Sdui). Brightwheel ist offen.
7. **Lesebestätigungen nach außen sind vorsichtig.** Intercom zeigt „Seen“ erst, wenn jemand zu antworten beginnt, damit Lesen, Zuweisen und Notizen keine Erwartung wecken. Famly macht „Seen by“ abschaltbar, gerade für Eltern. ParentSquare zeigt Eltern laut Auszug gar keine. Nur Sdui zeigt dem Verfasser eine Leseliste mit Namen. In allen belegten Fällen reicht eine einzige Person aus dem Team für die Bestätigung.

## Was das für moto heißt

Keine Entscheidung, nur Optionen. Heutiger Stand zum Vergleich: persönlicher Cursor, Antwort erledigt team-weit, Team-Markierung „ungelesen“, persönliches Sammel-Lesen.

### A. Beim heutigen Modell bleiben

- Pro: schon gebaut (#3654, #3663); entspricht Muster 1 und 2 (persönlich lesen, Antwort als Team-Erledigung).
- Contra: Kolleginnen und Kollegen ohne Elternnachrichten müssen die Zahl regelmäßig selbst wegklicken. Die Team-Markierung „ungelesen“ hat kein Vorbild in den geprüften Produkten und überlebt das persönliche Sammel-Lesen, was verwirren kann.

### B. Persönliche Einstellung „Elternnachrichten bei mir nicht zählen“

- Pro: entspricht Muster 4 (Front, Sdui, Stay Informed); löst den Kundenwunsch ohne Klickpflicht; keine Schul-Einstellung nötig.
- Contra: wer alles abschaltet, kann bei Ausfall der zuständigen Person nichts bemerken. Braucht eine Antwort, ob der persönliche Cursor weiter die Eltern-Lesebestätigung speist.

### C. Zuständigkeit auf Schulebene (Rolle oder Personen für Elternnachrichten)

- Pro: entspricht Muster 3 (Famly Favoriten, Procare Office Chat, Stay Informed Verwaltungsberechtigte). Zahl nur bei Zuständigen, ohne dass jede Person selbst etwas einstellen muss. Schul-Einstellung liefe über das Settings-System.
- Contra: neue Einstellung, neue Hilfe, Frage nach Vertretung. Ohne Zuständige fällt die Nachricht niemandem auf, also braucht es eine Rückfallregel (zum Beispiel alle zählen, wenn niemand zuständig ist).

### D. Zuweisung pro Unterhaltung

- Pro: stärkster Schutz vor Verstecken (Front: Archivieren durch andere lässt sie bei der zuständigen Person offen); klar, wer antworten soll.
- Contra: Helpdesk-Komplexität für eine OGS mit wenigen Nachrichten am Tag; mehr Bedienung, mehr Zustände, mehr Tests. Kein geprüftes Schul- oder Kita-Produkt dokumentiert das.

### E. Team-Zähler auf „offen“ statt „ungelesen“ umstellen

- Pro: entspricht Muster 2 (Help Scout zählt Active, Front bietet „Open“). Die Zahl sinkt, wenn jemand antwortet oder als erledigt markiert, nicht beim Öffnen. Macht eine Team-Aktion „für alle erledigt“ oder „wieder offen“ zur natürlichen Ergänzung und die Team-Markierung „ungelesen“ überflüssig.
- Contra: Wer nicht zuständig ist, sieht die Zahl weiterhin, bis jemand erledigt. Allein löst das den Kundenwunsch nicht; sinnvoll nur zusammen mit B oder C. „Erledigt ohne Antwort“ braucht eine Regel, wer das darf (Google: nur mit Berechtigung).

### Folgen für „Von der OGS gelesen“

- **Heute (jede Person öffnet):** entspricht Famly und Sdui. Risiko: Eltern halten die Sache für angekommen, obwohl nur eine nicht zuständige Person kurz geöffnet hat.
- **Nur Zuständige zählen:** passt zu C; das Lesen anderer bleibt intern.
- **Erst bei Antwort oder Antwortbeginn:** Muster Intercom; Lesen, Weiterreichen und Notizen wecken keine Erwartung. Contra: Eltern sehen länger nichts, auch wenn die Nachricht gesehen wurde.
- **Abschaltbar pro Schule:** Muster Famly; zusätzliche Schul-Einstellung.

Unabhängig von der Option bleiben diese Fragen für die Entscheidung in #3673: Wer darf team-weit etwas ändern (Rolle oder Berechtigung)? Holt eine neue Elternnachricht eine erledigte Unterhaltung zurück (heute bei Front, Help Scout, Sdui so)? Und bleibt der Zähler für Anfragen getrennt?

## Entscheidung

Die Entscheidung für moto steht in [ADR 0044](../adr/0044-elternnachrichten-zahl-persoenlich-lesebestaetigung-nur-beim-oeffnen.md): Zahl als persönliche Einstellung (alle, eigene Gruppen, keine), Lesebestätigung nur beim Öffnen.
