Use Machine ID lib for machine ID

Should token be passed with ALL GRPC messsages or just for login/register?
Could isolate other GRPC endpoints to ensure user/machine has been authed, including update stream

Only freely accessible endpoints available are Login/Register Peer and GetLoginURL
Other endpoints either check for jwt, or check machine ID has authenticated/logged in
