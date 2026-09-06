# KITA guardian privacy and change workflows

**Researched:** 2026-08-14\
**Scope:** Childcare, nursery, daycare, and kindergarten products only. This note compares documented behavior for guardian-entered attendance or pickup changes, child communication, separated-family privacy, approval, and notifications.\
**Evidence rule:** Only first-party product help and documentation is used. An undocumented behavior is marked as a gap rather than inferred.

## Executive findings

1. **There is no single market convention for co-guardian visibility.** Brightwheel explicitly gives every `Parent` contact access to all child-related messages, including another contact's schedule-change message and optional note. Famly explicitly keeps pickup facts visible even under its separated-parent privacy role, but its absence documentation does not say whether another guardian sees the absence note or its author.
2. **Communication privacy is usually recipient- or role-based, not field-based.** Famly can address both parents in a shared thread or one parent privately. Kinderpedia's daycare-oriented Quick Messages go to every active family member assigned to the child. None of the reviewed sources documents a separate privacy flag for the free-text reason attached to an operational change.
3. **Famly has the clearest separated-family control.** Its Limited Parent role hides the other guardian's identity and contact details and blocks one-to-one guardian messaging, while retaining access to child facts such as pickup times. Brightwheel and Kangarootime document general contact roles, but the reviewed official sources do not document a custody-specific mode.
4. **Approval is reserved mainly for capacity, payment, or consent.** Famly can approve or reject extra-care booking requests, while routine absence reporting immediately notifies the setting. Kinderpedia can automatically approve reported absences based on a configured notice threshold. Messaging products rely on delivery, push notifications, read state, acknowledgement, or an audit history more often than guardian-to-guardian approval.

## Product evidence

### Famly (nursery/childcare)

#### Guardian visibility into schedule, pickup, and reasons

- Famly's Limited Parent role **does not hide child operational information**. Limited parents retain full access to child-related information, explicitly including pickup times even when another contact will collect the child, accident reports, and holiday RSVPs. It hides the other parent's contact record, not shared facts about the child. ([Limited Parent Access](https://help.famly.co/en/articles/10701300-limited-parent-access))
- A parent can report `Sick`, `Absent`, or `Holiday`, specify the period, and add a note. The setting is notified immediately. **Gap:** the article does not state whether another guardian sees the category, note, author, or notification. ([Parents: Report Your Child's Absence](https://help.famly.co/en/articles/4912433-parents-report-your-child-s-absence))

#### Shared and private child communication

- A normal Parent role has full access to the child's profile, including the activity feed, tagged photos, private messages, invitations, and profile details. A Family role sees Newsfeed posts but not the child profile, activity feed, tagged profile photos, or observations. ([Assigning Roles to Children's Contacts](https://help.famly.co/en/articles/5323196-assigning-roles-to-children-s-contacts), [The Family Role Login](https://help.famly.co/en/articles/5219605-parents-the-family-role-login))
- For separated parents, staff can create separate restricted logins. A message addressed to both parents is visible to both; an individual message can be sent privately. In a multi-recipient group message, every recipient sees all replies; Famly does not support group delivery with separate private replies. ([Assigning Roles to Children's Contacts](https://help.famly.co/en/articles/5323196-assigning-roles-to-children-s-contacts), [Sending Private Messages to Parents](https://help.famly.co/en-us/articles/4912362-sending-private-messages-to-parents))
- The Newsfeed mixes audience types: settings may publish to all parents or a room, while observations and photos about an individual child can be visible only to that child's parents. ([Parents: The Newsfeed](https://help.famly.co/en/articles/4912431-parents-the-newsfeed))

#### Custody and separated-family controls

- The Limited Parent role is designed for separated parents. It works reciprocally: it hides the limited parent's name, phone number, and email from the other parent and hides the other parent's details from the limited parent. It also blocks adding contacts and starting one-to-one messages between the parents. Limited parents can still interact with the setting and can see the other's Newsfeed likes or comments. ([Limited Parent Access](https://help.famly.co/en/articles/10701300-limited-parent-access))
- Nursery staff alone assign the role, separately for each child. Staff must also check invoice-recipient settings because invoice access can expose another bill payer's address. Staff may still include separated parents in the same group conversation. Past direct conversations remain visible but locked. ([Limited Parent Access](https://help.famly.co/en/articles/10701300-limited-parent-access))
- **Gap:** the documentation calls this a privacy role, not a legal custody or court-order enforcement system. It does not document priority rules for conflicting guardian instructions.

#### Approval and notifications

- A nursery can configure extra-care bookings as requests, instant paid bookings, or both. Requests require staff approval; the parent receives email and in-app notice on approval or rejection. Instant paid bookings are confirmed without approval. ([Parent Bookings](https://help.famly.co/en-us/articles/13430530-parent-bookings), [Parents: Requesting Extra Care](https://help.famly.co/en-us/articles/14780531-parents-requesting-extra-care))
- Staff who follow the relevant room receive an in-app request notice and can approve or reject after checking capacity, staffing ratio, and payment history. This evidence covers added care, not ordinary pickup-time edits. ([Managing Parent Booking Requests](https://help.famly.co/en/articles/14624350-managing-parent-booking-requests))
- Permission answers can be locked after the first response so only staff can change them. Famly records who last changed an answer and when, and notifies staff who follow the room. ([Parental Permissions](https://help.famly.co/en/articles/4912250-parental-permissions))

### Brightwheel (childcare/daycare)

#### Guardian visibility into schedule, pickup, and reasons

- Every contact with the Parent role has full access to all messages about the child, whether another student contact or a staff member sent them. The documented message types include late drop-off, late pickup, early pickup, and absence, each with an optional note. This directly establishes that one Parent can see another contact's submitted schedule/pickup message and free text. ([Message Your Childcare Provider](https://help.mybrightwheel.com/en/articles/8436983-message-your-childcare-provider))
- At drop-off, a guardian can enter an expected pickup time and a custom note. The resulting check-in record appears in the child's feed for staff and parents. ([Everything You Need to Know About Student Check-in](https://help.mybrightwheel.com/en/articles/942374-everything-you-need-to-know-about-student-check-in))

#### Shared and private child communication

- Child message threads can target classroom Staff & Admins or Admins Only. That controls the staff audience; it does not make the message private from other Parent contacts on the child profile. ([Message Your Childcare Provider](https://help.mybrightwheel.com/en/articles/8436983-message-your-childcare-provider))
- Parent contacts can view the child's feed, messages, profile, and approved-pickup list. Family contacts can view feed activity and send messages but do not receive program-wide messages. Approved Pickup contacts can check the child in or out but cannot see child information or messages. ([Understand Student Contact Permissions](https://help.mybrightwheel.com/en/articles/1551943-understand-student-contact-permissions))

#### Custody and separated-family controls

- If the provider enables profile editing, a Parent can add, edit, or remove Parent, Family, Approved Pickup, and Emergency contacts. The provider is not notified when a contact is added. The provider setting applies school-wide and cannot be set for one user only. ([Manage Your Child's Contacts and Approved Pickup List](https://help.mybrightwheel.com/en/articles/1161627-manage-your-child-s-contacts-and-approved-pickup-list), [Understand Student Contact Permissions](https://help.mybrightwheel.com/en/articles/1551943-understand-student-contact-permissions))
- **Gap:** the reviewed official documentation does not describe a separated-parent or custody-specific role, a per-parent restriction, provider approval for pickup-list changes, or an alert to the other guardian. This is a documentation gap, not proof that no other safeguard exists.

#### Approval and notifications

- The cited schedule-message flow documents message delivery, not provider approval. The official contact-management article explicitly says providers are not alerted when a Parent adds a contact. **Gap:** the reviewed sources do not document co-guardian approval, co-guardian change alerts, or conflict handling for pickup and schedule messages.

### Kinderpedia (kindergarten/school platform; kindergarten-specific workflows cited)

#### Guardian visibility into schedule, pickup, and reasons

- Parents can send predefined `Absent`, `Early Pick Up`, `Late Pick Up`, `Early Drop Off`, and `Late Drop Off` Quick Messages. An absence includes a reason category and period; a personalized text message can also be supplied. ([How Does the Quick Message Module Work?](https://docs.kinderpedia.co/en/articles/3618401-how-does-the-quick-message-module-work), [How Do I Announce an Absence?](https://docs.kinderpedia.co/ro/articles/6911257-sunt-parinte-cum-anunt-ca-elevul-va-lipsi-folosind-mesaje-rapide))
- Quick Messages sent for a child go to all active family members assigned to that child. This documents family-wide receipt for the child-linked thread. **Gap:** the source does not explicitly distinguish whether every family member sees the submitting guardian's identity and every structured/free-text field in a pickup or absence message. ([How Does the Quick Message Module Work?](https://docs.kinderpedia.co/en/articles/3618401-how-does-the-quick-message-module-work))

#### Shared and private child communication

- Quick Messages are family-to-institution communication: staff choose the child rather than an individual parent, and the message reaches all active family accounts assigned to that child. When staff send the same message to several families, recipients are effectively blind-copied; one family's response is not visible to parents of other children. ([How Does the Quick Message Module Work?](https://docs.kinderpedia.co/en/articles/3618401-how-does-the-quick-message-module-work))
- Kinderpedia contrasts Quick Messages with Live Chat, which supports one-to-one and group communication. Every received Quick Message triggers a notification. ([How Does the Quick Message Module Work?](https://docs.kinderpedia.co/en/articles/3618401-how-does-the-quick-message-module-work))

#### Custody and separated-family controls

- Staff can assign family members a `Pickup Allow?` flag, which permits check-in/out by QR code. ([Family and Group Management](https://docs.kinderpedia.co/en/articles/3883568-family-and-group-management-how-to-create-remove-groups-and-add-families))
- **Gap:** the reviewed official documentation does not describe separated-family privacy, hiding one guardian from another, court-order/custody controls, or private child-linked Quick Messages to only one guardian.

#### Approval and notifications

- Every Quick Message produces a notification. Staff can see read/unread status; parents can inspect whether and when staff recipients opened their message. ([How Does the Quick Message Module Work?](https://docs.kinderpedia.co/en/articles/3618401-how-does-the-quick-message-module-work), [Guide for Parents](https://docs.kinderpedia.co/en/articles/5420302-guide-for-parents))
- A setting can automatically mark parent-reported absences approved when the report meets a configured minimum notice time. ([Automatically Approve Absences Announced via Quick Messages](https://docs.kinderpedia.co/en/articles/10057525-how-can-i-set-absences-announced-via-quick-messages-to-be-automatically-approved))
- **Gap:** the source does not document what happens to reports outside the automatic-approval threshold or whether another guardian receives an approval-state notification.

### Kangarootime / KT Connect (childcare)

#### Guardian visibility and communication

- Primary Contacts have full authority, including access to daily activities, photos, and student/account messaging; a child can have multiple Primary Contacts. Authorized Contacts receive per-child permissions for pickup, activity viewing, messaging, and billing. ([Primary vs. Authorized Contacts Permissions](https://help.kangarootime.com/hc/en-us/articles/12773986878868-Primary-vs-Authorized-Contacts-Permissions))
- Classroom messages and acknowledgements are available to the child's Primary Contacts and support push notifications. Parents can send free text to the professional currently caring for the child and can acknowledge a staff message; staff receive notice of the acknowledgement. ([Student Messaging from the Classroom](https://help.kangarootime.com/hc/en-us/articles/15706057426196-Student-Messaging-from-the-Classroom), [How to Use Student Messaging in KT Connect](https://help.kangarootime.com/hc/en-us/articles/15607485377556-How-to-use-Student-Messaging-in-KT-Connect))
- **Gap:** the sources do not say whether one Primary Contact sees which other guardian authored a message or attendance edit, nor whether free-text reasons are shared between Primary Contacts.

#### Custody, approval, and notifications

- Parents can update future attendance using a predefined absence reason. Staff create additional contacts and choose their permissions. ([Parent Frequently Asked Questions](https://help.kangarootime.com/hc/en-us/articles/12774023133716-Parent-Frequently-Asked-Questions))
- **Gap:** the reviewed docs do not describe separated-family controls, an approval flow for ordinary attendance changes, co-guardian notifications, or conflict handling.

## Design implications for moto

These are product-design conclusions from the documented patterns, not claims about vendor behavior:

1. **Model child facts separately from guardian-private data.** Famly provides the strongest precedent for sharing a child's operative pickup state while hiding guardian contact details and direct conversations.
2. **Choose and state the free-text rule explicitly.** Brightwheel shares schedule-message notes with every full Parent; other products leave this unclear. A reason may contain health, relationship, or safety information, so inheriting visibility from the schedule fact without a deliberate rule is risky.
3. **Use recipient-explicit communication.** A shared child thread and a private guardian-to-setting thread should look different before send. Famly's rule that all group recipients see all replies is simple and predictable.
4. **Put approval where the institution must commit resources or consent matters.** Extra care can require approval; routine pickup/absence changes can instead use immediate notification, an audit trail, and a clear final state.
5. **Treat custody controls as staff-managed, per child, and auditable.** Do not let one guardian weaken another guardian's privacy or access. Also audit indirect leaks such as invoices, notification previews, contact lists, author labels, and historical threads.

## Unanswered questions across the reviewed products

- Whether a co-guardian receives a push/email notification when another guardian changes attendance or pickup.
- Whether the submitting guardian's identity is shown beside a structured change.
- Whether free-text reasons can have narrower visibility than the operative schedule fact.
- How simultaneous or contradictory guardian instructions are resolved.
- How court orders, sole custody, restricted collection, and emergency overrides are represented and audited.
