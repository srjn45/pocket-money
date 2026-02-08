We are building the "PocketMoneyApp" which will be supported on web, android & ios

it will be built using the react native & expo

this app will contain following screen
 - Login Screen
 - Register Screen
 - Group Listing Screen
 - Group Detail
 - Chores Screen
 - Profile Screen


## Login Screen
Login Screen will have input fields for email & password and a Login Button

below the login button there should be a link to register a new user, which should redirect to Register Screen

### Login Button
on click of login button it should perform FE validation that both email & password are mandatory, in case of validation failure an appropriate error message should be displayed

upon successful FE validation, it should call login api provided by auth service

depending on failure or success response, it should eitehr either display the error message or save the token & user details and redirect the user to Group Listing Screen

## Group Listing Screen
this screen should have button to create a group & join a group

followed by sectioned list
 - section 1 should be the list of groups where the current user is head of the group
    - each group item in this list should have group name & the member count & sum of total amount owed to all the members
 - section 2 should be the list of groups where the user if in member role 
    - each item in this list should have group name & the sum of ledger amount for the group for current user
 - on click of any list item user should be redirected to Group Detail Screen

### create group button
 this should open an overlay with a single input field for group name & a button to create group (on click of which it should validate the group name is not empty and then make the api call)

### join group button
 on click of this button open an overlay with input field for the invite link and a join button (on click of which again perform FE validation & then call the group join api)


## Group Detail Screen
this screen will have 2 tabs, detail & chores

detail tab will open the main group screen, while the chores tab will open the chores screen for that group

the detail screen should be as followes

this screen will look different for head & member role

there should be a button to add the ledger entry, (which should open the form in an overlay with the required fields) and on submit it should call the create ledger api

for head user
 - there should be a button to get the invite link for the group, this should work differently on web & mobile
on web it should simply copy the invite link (with a toast that says "link copied")
on mobile it should open a share option for user to select any social media/messenger app to share the link, (the link should be auto populated)

 - it'll contain the list of group members with their names & sum of total amount as per their respective ledger
 - in case of ledger entry in pending_validation state it should have button to approve or reject it
 - rejected entries should be strikedthrough & should not contribute to total sum
 - only approved entries should be contributing to the total sum

for member user
 - it should directly contain the list of ledger entry
 - with indecations for approved rejected & pending approval state

## Chores Screen
head user can create chores for a group,

eaqch chore will have name description & amount

member users can only see the list of chores they cannot create or modify the chores

## Profile Screen
current screen is good enough with user details & logout button I do not want to make changes in that for now.