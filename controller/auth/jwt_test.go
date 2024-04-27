package auth

const TokenLength = 260

// func Test_GenerateJWT(t *testing.T) {
// 	token, err := GenerateJwtWithClaims("100")
// 	assert.Nil(t, err, "error generating token: ", err)
// 	assert.NotEqualValues(t, token, "", "token not generated")
// 	// assert.Equal(t, TokenLength, len(token), "token is improper length")
// }
//
// func Test_ValidateJWT(t *testing.T) {
// 	token, _ := GenerateJwtWithClaims("100")
// 	claims, err := ParseJwtWithClaims(token)
// 	assert.Nil(t, err, "error validating token: ", err)
// 	assert.NotNil(t, claims, "returned claims are nil")
// }
