package main

// Notifications contain all notif
type Notifications struct {
	HaveNotfications bool
	Notifications    []Notification
}

// Notification struct
type Notification struct {
	Title string
	Text  string
	Type  string
}

// AddNotification add notification to display on template
func AddNotification(Ntitle, Ntext, Ntype string, N *Notifications) {

	N.HaveNotfications = true
	N.Notifications = append(N.Notifications, Notification{
		Title: Ntitle,
		Text:  Ntext,
		Type:  Ntype,
	})
	return
}

// ClearNotification remove all
func ClearNotification(N *Notifications) {

	N.HaveNotfications = false
	N.Notifications = nil
	return
}
