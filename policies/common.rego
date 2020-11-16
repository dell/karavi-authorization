package karavi.common

default roles = {}
roles = {
	"CSIBronze": {
		"pools": ["bronze"],
		"quota": 9000000
	},
	"CSISilver": {
		"pools": ["silver"],
		"quota": 16000000
	},
	"CSIGold": {
		"pools": ["gold"],
		"quota": 32000000
	}
}
