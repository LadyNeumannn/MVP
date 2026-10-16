package menu

import "github.com/ladyneumannn/avito-kuhnya/services/restaurant-example/internal/kuhnyaclient"

func Starter() []kuhnyaclient.MenuItemInput {
	return []kuhnyaclient.MenuItemInput{
		{
			ID:          "6f2f8c1a-0001-4b1a-9c3a-000000000001",
			Name:        "Пельмени домашние",
			Description: "Свинина/говядина, подаются со сметаной",
			PriceCents:  35000,
			Available:   true,
		},
		{
			ID:          "6f2f8c1a-0001-4b1a-9c3a-000000000002",
			Name:        "Вареники с картошкой",
			Description: "12 штук, с жареным луком",
			PriceCents:  29000,
			Available:   true,
		},
		{
			ID:          "6f2f8c1a-0001-4b1a-9c3a-000000000003",
			Name:        "Борщ",
			Description: "500 мл, с пампушками",
			PriceCents:  25000,
			Available:   true,
		},
		{
			ID:          "6f2f8c1a-0001-4b1a-9c3a-000000000004",
			Name:        "Компот",
			Description: "Сезонный, 400 мл",
			PriceCents:  12000,
			Available:   false,
		},
	}
}
