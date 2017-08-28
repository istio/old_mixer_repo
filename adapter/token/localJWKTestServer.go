package token

import (

	"crypto/rsa"
	"net/http"
	"math/big"
	"encoding/json"
	"github.com/dgrijalva/jwt-go"
	"time"
	"math/rand"
	cryptoRand "crypto/rand"
)

/*
A local JWK server used for testing the adapter. serves keys according to the JWK RFC https://tools.ietf.org/html/rfc7517
 */

const (
	test_server_port = "21356"
	test_server_keys_location = "/keys" //localhost
	test_server_addr = "http://localhost:"+test_server_port+test_server_keys_location
	test_server_iss_name="local"
	//crypto testing consts:
	e1 = 65537
	n1 = "18738892009081519719845870431266951090046656171854280447005872078714046821952756050537228138106196550752508022019816731555094189004287665983819001535160668468594740189745339028257267968871048339993040577699838638277550476563233572454949402966399558670387670403570634026189554182981838554821158102488698440045977277902240961498131426869260113477807234364105166861405097632790020767744845422811128659685312246833069948729848729479918680024898335255952043297617273178616626827576955145484012521905279078436587772766821541705010996142708734415088364299426473567014523363379271183494202325991841529645373006457112060098691"
	d1 = "450623217515487054312481373875470572621778997006917405198304078551861357575072760969325290227739532843827954326612923523060690020458021599867231432922062552550548705907176927667324630650484034725865266192455341164920877535798954944367307918809919655530936242977666344588167560193163824441127075842992336260616578549868695069084255035372638508617331247617855748686174305491692105123111908877677031643571626669535172414500727437213154924271748239398275022701288983617408179488878471389366581431859811133126440057869486820275198177720710216591859678216269566798431523359967046398157491673547936096993370444401599533505"
	pf11 = "137483878809329042769707446870118861832684600046359643704369199548839215785525177146682858096094687372070938113731809940077887693595291334614103772697328221561165965309100428634939799182219205434922484431751230329493730682288646840164701105480168197718720255252997245082172606141223207712231322182003568420999"
	pf12 = "136298831334761425809167231262704045381886074254611658280801111534947101798542852189623705888231711084522745051945489303395021854147228968644570132296609855955156383103750415170825484588671267524693523060582647834669763573819716087118645597969364633719149081195969166967837123524913316864198461676918371990309"
	id1= "1"
	e2 = 65537
	n2 = "26658957953236697552708592674599081539601909769135437983264396336208232986117022830573184468855487390454112595615638924035406523350193981653873954952458916378588190665393364059361957986455214182136446330546485792402548829317462029468047065758378998687394513321794064879425565607819155476399205755120729084609536540721277798169287677392646632336356691536716467336517077072971573775572197320061768310457236440835427606212147055095456081892397513460973811116465596136477523876533292387786887605271603371018145368387047488479432458660323845811456961985117210185715951453441833297390458575874612717922520361214054854152663"
	pf21 = "166723200685882014193571146441857183092884521548753919894720712548455582909203385592343431387466127009946781640017314327837113563514516014395696360157512857177448158800569092433547776591271660671246201581702349700040522096081358932119233864019396529966018871375008447485530175912849622896260633181333330293787"
	pf22 = "159899509147884038632862931597948599270924347531228992364878986822592193680714224767384304053858742301471116154931570373700983478968478282953925017232663946607129278051489510801378158804603842299070848025853283091336478855290464481487032201709094752992371420467698434292476967920390192649972794486283447983349"
	d2 = "5310883242099246582055379189764035713887461036450131655545721631529284066508138152127248675181611049785142652980114771689370394874042641934677790497876064089580655466795485926408436813878561367807092837505758861492098776501347090296089575210055330681334555532437299709565286549211695590275234300301451682693102288624096514691128949947594591261255948183601048882968485353155140873746321068859878263384160725663632310802980595887254424374025115006794290572247259941828876950676384153853707242655255856161137167248806714800238827774357654522275030683789906678584657804382840143464549275931644179828567102619623960795137"
	id2 = "2"
	expDuration = 30000000 * time.Minute
)

type (
	test_jwk_server struct{
		privKeys map[string]*rsa.PrivateKey //maps from key id to private key
		_status bool //whether the server finished initializing, and isn't closed
	}
)

func newTestServer() *test_jwk_server{
	var N1,_ = (&big.Int{}).SetString(n1,10)
	var PF11,_ = (&big.Int{}).SetString(pf11,10)
	var PF12,_ = (&big.Int{}).SetString(pf12,10)
	var D1,_ = (&big.Int{}).SetString(d1,10)
	var N2,_ = (&big.Int{}).SetString(n2,10)
	var PF21,_ = (&big.Int{}).SetString(pf21,10)
	var PF22,_ = (&big.Int{}).SetString(pf22,10)
	var D2,_ = (&big.Int{}).SetString(d2,10)
	var priv1 = &rsa.PrivateKey{rsa.PublicKey{N:N1,E:e1},D1,[]*big.Int{PF11,PF12},rsa.PrecomputedValues{}}
	var priv2 = &rsa.PrivateKey{rsa.PublicKey{N:N2,E:e2},D2,[]*big.Int{PF21,PF22},rsa.PrecomputedValues{}}

	return &test_jwk_server{map[string]*rsa.PrivateKey{id1:priv1,id2:priv2},false}
}

func(ts *test_jwk_server) start(){
	keySet := keySet{Keys: make([]key,0)}
	for kid,priv := range ts.privKeys{
		key := key{Kty:"RSA",Kid:kid,Alg:"RS256"}
		key.putRSAPublicKey(&priv.PublicKey)
		keySet.Keys = append(keySet.Keys,key)
	}
	//starts the test server, listening for requests for the JWKs
	http.HandleFunc(test_server_keys_location, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(keySet)
	})
	ts._status=true
	http.ListenAndServe(":"+test_server_port, nil)
}

func (ts *test_jwk_server) status() bool {
	return ts._status
}

func(ts *test_jwk_server) stop(){
	ts._status = false
	//stops the test server
}

func (ts *test_jwk_server) get_kid_at_random() (kid string) {
	rand.Seed(time.Now().Unix())
	i := rand.Intn(len(ts.privKeys))
	var k string
	for k = range ts.privKeys {
		if i == 0 {
			break
		}
		i--
	}
	return k
}

func (ts *test_jwk_server) get_valid_no_claims_token() string {
	//build the test token:
		claims := make(jwt.MapClaims)
		claims["iss"] = test_server_iss_name //token issued by test issuer
		claims["exp"] = time.Now().Add(expDuration).Unix() //not expired
		testToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		kid := ts.get_kid_at_random()
		testToken.Header["kid"] = kid
		tokenString, _ := testToken.SignedString(ts.privKeys[kid])
		return tokenString
}

func (ts *test_jwk_server) get_expired_token() string {
	//build the test token:
	claims := make(jwt.MapClaims)
	claims["iss"] = test_server_iss_name //token issued by test issuer
	claims["exp"] = time.Now().Add(-30 * time.Minute).Unix() //expired
	testToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	kid := ts.get_kid_at_random()
	testToken.Header["kid"] = kid
	tokenString, _ := testToken.SignedString(ts.privKeys[kid])
	return tokenString
}

func (ts *test_jwk_server) get_late_nbf_token() string {
	//build the test token:
	claims := make(jwt.MapClaims)
	claims["iss"] = test_server_iss_name //token issued by test issuer
	claims["exp"] = time.Now().Add(expDuration).Unix() //not expired
	claims["nbf"] = time.Now().Add(20 * time.Minute).Unix() //not valid yet
	testToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	kid := ts.get_kid_at_random()
	testToken.Header["kid"] = kid
	tokenString, _ := testToken.SignedString(ts.privKeys[kid])
	return tokenString
}


func (ts *test_jwk_server) get_no_iss_token() string {
	//build the test token:
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(expDuration).Unix() //not expired
	testToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	kid := ts.get_kid_at_random()
	testToken.Header["kid"] = kid
	tokenString, _ := testToken.SignedString(ts.privKeys[kid])
	return tokenString
}

func (ts *test_jwk_server) get_no_kid_token() string {
	//build the test token:
	claims := make(jwt.MapClaims)
	claims["iss"] = test_server_iss_name //token issued by test issuer
	claims["exp"] = time.Now().Add(expDuration).Unix() //not expired
	testToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	kid := ts.get_kid_at_random()
	tokenString, _ := testToken.SignedString(ts.privKeys[kid])
	return tokenString
}

func (ts *test_jwk_server) get_no_exp_token() string {
	//build the test token:
	claims := make(jwt.MapClaims)
	claims["iss"] = test_server_iss_name //token issued by test issuer
	testToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	kid := ts.get_kid_at_random()
	testToken.Header["kid"] = kid
	tokenString, _ := testToken.SignedString(ts.privKeys[kid])
	return tokenString
}

func (ts *test_jwk_server) get_invalid_token() string {
	//build the test token:
	claims := make(jwt.MapClaims)
	claims["iss"] = test_server_iss_name //token issued by test issuer
	claims["exp"] = time.Now().Add(expDuration).Unix() //not expired
	testToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	kid := ts.get_kid_at_random()
	testToken.Header["kid"] = kid
	privatek, _ := rsa.GenerateKey(cryptoRand.Reader, 2048)
	tokenString, _ := testToken.SignedString(privatek)//token signed by arbitrary key
	return tokenString
}


func (ts *test_jwk_server) get_valid_with_claims_token() string {
	//build the test token:
	claims := make(jwt.MapClaims)
	claims["iss"] = test_server_iss_name //token issued by test issuer
	claims["exp"] = time.Now().Add(expDuration).Unix() //not expired
	claims["locations"] = make(map[string](map[string]int))
	locations := claims["locations"].(map[string](map[string]int))
	locations["us"] = map[string]int{
		"ca": 3,
		"ny": 5,
		"dc": 10,
	}
	locations["israel"] = map[string]int{
		"haifa": 3,
		"tel-aviv": 5,
	}
	claims["servers"] = map[string][]string{
		"ny": []string{"jimmy","carl","eliot"},
		"haifa": []string{"jake","roey"},
		"ca": []string{},
	}
	claims["name"] = "johnDoe"
	testToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	kid := ts.get_kid_at_random()
	testToken.Header["kid"] = kid
	tokenString, _ := testToken.SignedString(ts.privKeys[kid])
	return tokenString
}




