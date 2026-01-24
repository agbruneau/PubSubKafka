/*
Package models defines the shared domain entities and data structures for the Kafka Order Tracking System.

This package contains the core business logic, validation rules, and data transfer objects (DTOs)
used across the Producer, Tracker, and Monitor services.

Key Concepts:
- **Event Carried State Transfer (ECST)**: The `Order` struct is a self-contained event carrying
  all necessary state (Customer, Inventory, Payment) to allow downstream services to function autonomously.
- **Rich Domain Model**: Entities include built-in validation logic (`Validate()`) to ensure data integrity
  at the boundaries of the system.
*/
package models

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Validation errors
var (
	ErrEmptyOrderID      = errors.New("order_id est requis")
	ErrInvalidSequence   = errors.New("sequence doit être positif")
	ErrEmptyStatus       = errors.New("status est requis")
	ErrNoItems           = errors.New("au moins un article est requis")
	ErrInvalidTotal      = errors.New("le total ne correspond pas aux montants calculés")
	ErrEmptyCustomerID   = errors.New("customer_id est requis")
	ErrEmptyCustomerName = errors.New("le nom du client est requis")
	ErrInvalidEmail      = errors.New("l'email n'est pas valide")
	ErrEmptyItemID       = errors.New("item_id est requis")
	ErrEmptyItemName     = errors.New("le nom de l'article est requis")
	ErrInvalidQuantity   = errors.New("la quantité doit être positive")
	ErrInvalidPrice      = errors.New("le prix doit être positif ou nul")
	ErrInvalidItemTotal  = errors.New("le total de l'article ne correspond pas au calcul")
)

// emailRegex provides basic email validation using standard patterns.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// CustomerInfo contient les informations détaillées et complètes du client.
// Ces données sont embarquées dans chaque message de commande.
type CustomerInfo struct {
	CustomerID   string `json:"customer_id"`   // Identifiant unique du client.
	Name         string `json:"name"`          // Nom complet du client.
	Email        string `json:"email"`         // Adresse email du client.
	Phone        string `json:"phone"`         // Numéro de téléphone du client.
	Address      string `json:"address"`       // Adresse de livraison principale du client.
	LoyaltyLevel string `json:"loyalty_level"` // Niveau de fidélité (ex: "gold", "silver").
}

// InventoryStatus représente l'état de l'inventaire pour un article spécifique au moment de la commande.
// Ces informations permettent de comprendre le contexte du stock sans interroger un service d'inventaire.
type InventoryStatus struct {
	ItemID       string  `json:"item_id"`       // Identifiant unique de l'article.
	ItemName     string  `json:"item_name"`     // Nom de l'article.
	AvailableQty int     `json:"available_qty"` // Quantité disponible en stock au moment de la commande.
	ReservedQty  int     `json:"reserved_qty"`  // Quantité réservée spécifiquement pour cette commande.
	UnitPrice    float64 `json:"unit_price"`    // Prix unitaire de l'article.
	InStock      bool    `json:"in_stock"`      // Indicateur booléen si l'article était en stock.
	Warehouse    string  `json:"warehouse"`     // Entrepôt d'où l'article est expédié.
}

// OrderItem représente un article individuel au sein d'une commande.
type OrderItem struct {
	ItemID     string  `json:"item_id"`     // Identifiant unique de l'article.
	ItemName   string  `json:"item_name"`   // Nom de l'article.
	Quantity   int     `json:"quantity"`    // Quantité de cet article commandée.
	UnitPrice  float64 `json:"unit_price"`  // Prix unitaire de l'article.
	TotalPrice float64 `json:"total_price"` // Prix total pour cette ligne d'article (Quantity * UnitPrice).
}

// OrderMetadata contient les métadonnées techniques et contextuelles de l'événement de commande.
// Ces informations sont essentielles pour le suivi, le débogage et l'analyse des flux de messages.
type OrderMetadata struct {
	Timestamp     string `json:"timestamp"`      // Horodatage de la création de l'événement au format ISO 8601 (UTC).
	Version       string `json:"version"`        // Version du schéma du message, utile pour la gestion des évolutions.
	EventType     string `json:"event_type"`     // Type d'événement (ex: "order.created", "order.updated").
	Source        string `json:"source"`         // Service ou application ayant généré l'événement (ex: "order-service").
	CorrelationID string `json:"correlation_id"` // Identifiant unique pour suivre un flux de travail à travers plusieurs services.
}

// Order est la structure principale qui représente une commande client complète.
// Elle agrège toutes les informations nécessaires à son traitement en un seul message,
// conformément au principe de l'Event Carried State Transfer.
type Order struct {
	// --- Identifiants principaux ---
	OrderID  string `json:"order_id"` // Identifiant unique de la commande (UUID).
	Sequence int    `json:"sequence"` // Numéro séquentiel pour suivre l'ordre de production des messages.

	// --- État et détails de la commande ---
	Status          string      `json:"status"`           // Statut actuel de la commande (ex: "pending", "shipped").
	Items           []OrderItem `json:"items"`            // Liste complète des articles de la commande.
	SubTotal        float64     `json:"sub_total"`        // Sous-total de la commande (somme des TotalPrice des articles).
	Tax             float64     `json:"tax"`              // Montant des taxes appliquées.
	ShippingFee     float64     `json:"shipping_fee"`     // Frais de livraison.
	Total           float64     `json:"total"`            // Montant total de la commande (SubTotal + Tax + ShippingFee).
	Currency        string      `json:"currency"`         // Devise utilisée pour la transaction (ex: "EUR").
	PaymentMethod   string      `json:"payment_method"`   // Méthode de paiement (ex: "credit_card").
	ShippingAddress string      `json:"shipping_address"` // Adresse de livraison complète pour cette commande.

	// --- Données enrichies (ECST) ---
	Metadata        OrderMetadata     `json:"metadata"`         // Métadonnées techniques de l'événement.
	CustomerInfo    CustomerInfo      `json:"customer_info"`    // Informations complètes sur le client.
	InventoryStatus []InventoryStatus `json:"inventory_status"` // État de l'inventaire pour chaque article au moment de la commande.
}

// Validate vérifie que les informations client sont valides.
func (c *CustomerInfo) Validate() error {
	if strings.TrimSpace(c.CustomerID) == "" {
		return ErrEmptyCustomerID
	}
	if strings.TrimSpace(c.Name) == "" {
		return ErrEmptyCustomerName
	}
	if c.Email != "" && !emailRegex.MatchString(c.Email) {
		return ErrInvalidEmail
	}
	return nil
}

// Validate vérifie qu'un article de commande est valide.
func (i *OrderItem) Validate() error {
	if strings.TrimSpace(i.ItemID) == "" {
		return ErrEmptyItemID
	}
	if strings.TrimSpace(i.ItemName) == "" {
		return ErrEmptyItemName
	}
	if i.Quantity <= 0 {
		return ErrInvalidQuantity
	}
	if i.UnitPrice < 0 {
		return ErrInvalidPrice
	}
	// Vérifier que le total correspond au calcul (avec une tolérance pour les erreurs d'arrondi)
	expectedTotal := float64(i.Quantity) * i.UnitPrice
	if math.Abs(i.TotalPrice-expectedTotal) > 0.01 {
		return fmt.Errorf("%w: attendu %.2f, obtenu %.2f", ErrInvalidItemTotal, expectedTotal, i.TotalPrice)
	}
	return nil
}

// Validate ensures the order is logically correct and semantically valid.
// It verifies all required fields and performs financial consensus checks.
func (o *Order) Validate() error {
	// Valider les identifiants
	if strings.TrimSpace(o.OrderID) == "" {
		return ErrEmptyOrderID
	}
	if o.Sequence <= 0 {
		return ErrInvalidSequence
	}
	if strings.TrimSpace(o.Status) == "" {
		return ErrEmptyStatus
	}

	// Valider les articles
	if len(o.Items) == 0 {
		return ErrNoItems
	}

	var calculatedSubTotal float64
	for idx, item := range o.Items {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("article %d: %w", idx, err)
		}
		calculatedSubTotal += item.TotalPrice
	}

	// Vérifier la cohérence des montants (avec tolérance pour erreurs d'arrondi)
	if math.Abs(o.SubTotal-calculatedSubTotal) > 0.01 {
		return fmt.Errorf("sous-total incohérent: attendu %.2f, obtenu %.2f", calculatedSubTotal, o.SubTotal)
	}

	expectedTotal := o.SubTotal + o.Tax + o.ShippingFee
	if math.Abs(o.Total-expectedTotal) > 0.01 {
		return fmt.Errorf("%w: attendu %.2f, obtenu %.2f", ErrInvalidTotal, expectedTotal, o.Total)
	}

	// Valider les informations client
	if err := o.CustomerInfo.Validate(); err != nil {
		return fmt.Errorf("informations client: %w", err)
	}

	return nil
}

// IsValid returns true if the order passes all validation checks.
func (o *Order) IsValid() bool {
	return o.Validate() == nil
}
