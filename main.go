package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type Config struct {
	ImagenBase         string
	BoletasPorFila     int
	NumeroMinimo       int
	NumeroMaximo       int
	BoletasPorPagina   int
	CantidadPaginas    int
	CarpetaSalida      string
	AnchoTalonario     int
	AltoTalonario      int
	MargenSuperior     int
	MargenInferior     int
	MargenIzquierdo    int
	MargenDerecho      int
	ColorTexto         color.RGBA
	ColorBorde         color.RGBA
	Fuente             font.Face
	RutaFuente         string
	TamanoFuente       float64
	AnchoLineas        int
	OrientacionBoletas int // 0: izquierda, 1: centro, 2: derecha
}

type Boleta struct {
	Numero     int
	Formateado string
}

type Talonario struct {
	ID      int
	Boletas []Boleta
}

type GeneradorTalonarios struct {
	config         Config
	numerosUsados  map[int]bool
	imagenBase     image.Image
	digitosFormato int
}

func NewGeneradorTalonarios(config Config) (*GeneradorTalonarios, error) {
	gen := &GeneradorTalonarios{
		config:        config,
		numerosUsados: make(map[int]bool),
	}

	gen.digitosFormato = len(strconv.Itoa(config.NumeroMaximo))

	if err := gen.validarConfig(); err != nil {
		return nil, err
	}

	if config.RutaFuente != "" {
		if err := gen.cargarFuentePersonalizada(); err != nil {
			fmt.Printf("⚠️  Advertencia: No se pudo cargar la fuente personalizada (%v), usando fuente por defecto\n", err)
			gen.config.Fuente = basicfont.Face7x13
		}
	} else {
		gen.config.Fuente = basicfont.Face7x13
	}

	if config.ImagenBase != "" {
		if err := gen.cargarImagenBase(); err != nil {
			return nil, fmt.Errorf("error cargando imagen base: %v", err)
		}
	}

	if err := os.MkdirAll(config.CarpetaSalida, 0755); err != nil {
		return nil, fmt.Errorf("error creando carpeta de salida: %v", err)
	}

	return gen, nil
}

func (g *GeneradorTalonarios) cargarFuentePersonalizada() error {
	if _, err := os.Stat(g.config.RutaFuente); os.IsNotExist(err) {
		return fmt.Errorf("el archivo de fuente no existe: %s", g.config.RutaFuente)
	}

	fontBytes, err := os.ReadFile(g.config.RutaFuente)
	if err != nil {
		return fmt.Errorf("error leyendo archivo de fuente: %v", err)
	}

	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return fmt.Errorf("error parseando fuente: %v", err)
	}

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    g.config.TamanoFuente,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return fmt.Errorf("error creando face de fuente: %v", err)
	}

	g.config.Fuente = face
	fmt.Printf("✅ Fuente personalizada cargada: %s (tamaño: %.1f)\n", g.config.RutaFuente, g.config.TamanoFuente)
	return nil
}

func (g *GeneradorTalonarios) validarConfig() error {
	totalNumeros := g.config.NumeroMaximo - g.config.NumeroMinimo + 1
	numerosNecesarios := g.config.BoletasPorPagina * g.config.CantidadPaginas

	if numerosNecesarios > totalNumeros {
		return fmt.Errorf("no hay suficientes números: necesitas %d pero solo hay %d disponibles",
			numerosNecesarios, totalNumeros)
	}

	if g.config.BoletasPorPagina <= 0 || g.config.CantidadPaginas <= 0 {
		return errors.New("la cantidad de boletas y páginas debe ser mayor a 0")
	}

	if g.config.BoletasPorFila <= 0 {
		return errors.New("el número de boletas por fila debe ser mayor a 0")
	}

	if g.config.MargenSuperior < 0 || g.config.MargenInferior < 0 ||
		g.config.MargenIzquierdo < 0 || g.config.MargenDerecho < 0 {
		return errors.New("los márgenes deben ser positivos o cero")
	}

	return nil
}

func (g *GeneradorTalonarios) cargarImagenBase() error {
	file, err := os.Open(g.config.ImagenBase)
	if err != nil {
		return err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(g.config.ImagenBase))
	switch ext {
	case ".jpg", ".jpeg":
		g.imagenBase, err = jpeg.Decode(file)
	case ".png":
		g.imagenBase, err = png.Decode(file)
	default:
		g.imagenBase, _, err = image.Decode(file)
	}

	return err
}

func (g *GeneradorTalonarios) generarNumeroAleatorio() int {
	for {
		numero := rand.Intn(g.config.NumeroMaximo-g.config.NumeroMinimo+1) + g.config.NumeroMinimo
		if !g.numerosUsados[numero] {
			g.numerosUsados[numero] = true
			return numero
		}
	}
}

func (g *GeneradorTalonarios) formatearNumero(numero int) string {
	formato := fmt.Sprintf("%%0%dd", g.digitosFormato)
	return fmt.Sprintf(formato, numero)
}

func (g *GeneradorTalonarios) crearTalonario(id int) Talonario {
	talonario := Talonario{
		ID:      id,
		Boletas: make([]Boleta, g.config.BoletasPorPagina),
	}

	for i := range g.config.BoletasPorPagina {
		numero := g.generarNumeroAleatorio()
		talonario.Boletas[i] = Boleta{
			Numero:     numero,
			Formateado: g.formatearNumero(numero),
		}
	}

	return talonario
}

func (g *GeneradorTalonarios) crearImagenTalonario(talonario Talonario) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, g.config.AnchoTalonario, g.config.AltoTalonario))

	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

	if g.imagenBase != nil {
		imagenEscalada := g.escalarImagen(g.imagenBase, g.config.AnchoTalonario, g.config.AltoTalonario)
		draw.Draw(img, img.Bounds(), imagenEscalada, image.Point{}, draw.Over)
	}

	filas := (len(talonario.Boletas) + g.config.BoletasPorFila - 1) / g.config.BoletasPorFila

	anchoBoleta := (g.config.AnchoTalonario - g.config.MargenDerecho - g.config.MargenIzquierdo) / g.config.BoletasPorFila
	altoBoleta := (g.config.AltoTalonario - g.config.MargenSuperior - g.config.MargenInferior) / filas

	g.dibujarLineaSuperior(img, g.config.MargenIzquierdo, g.config.MargenSuperior, g.config.ColorBorde)

	for i, boleta := range talonario.Boletas {
		fila := i / g.config.BoletasPorFila
		columna := i % g.config.BoletasPorFila

		x := (columna * anchoBoleta) + g.config.MargenIzquierdo
		y := fila*altoBoleta + g.config.MargenSuperior

		g.dibujarBoleta(img, boleta, x, y, anchoBoleta, altoBoleta)
	}

	return img
}

func (g *GeneradorTalonarios) escalarImagen(src image.Image, ancho, alto int) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, ancho, alto))

	scaleX := float64(bounds.Dx()) / float64(ancho)
	scaleY := float64(bounds.Dy()) / float64(alto)

	for y := range alto {
		for x := range ancho {
			srcX := int(float64(x) * scaleX)
			srcY := int(float64(y) * scaleY)
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	return dst
}

func (g *GeneradorTalonarios) dibujarBoleta(img *image.RGBA, boleta Boleta, x, y, ancho, alto int) {

	advance := font.MeasureString(g.config.Fuente, "0")
	anchoCaracter := advance.Round()
	bordeColor := g.config.ColorBorde
	g.dibujarRectangulo(img, x, y, ancho, alto, bordeColor)
	if g.config.OrientacionBoletas == 0 { // Izquierda
		g.dibujarTexto(img, boleta.Formateado, x+anchoCaracter, y+alto/2, g.config.ColorTexto)
	}
	if g.config.OrientacionBoletas == 1 { // Izquierda
		g.dibujarTexto(img, boleta.Formateado, x+(ancho/2)-anchoCaracter*g.digitosFormato/2, y+alto/2, g.config.ColorTexto)
	}
	if g.config.OrientacionBoletas == 2 { // Izquierda
		g.dibujarTexto(img, boleta.Formateado, x+ancho/g.config.BoletasPorFila-anchoCaracter*(g.digitosFormato+1), y+alto/2, g.config.ColorTexto)
	}
}

func (g *GeneradorTalonarios) dibujarLineaSuperior(img *image.RGBA, x, y int, col color.RGBA) {
	for i := range g.config.AnchoTalonario - g.config.MargenIzquierdo - g.config.MargenDerecho {
		if x+i >= img.Bounds().Max.X {
			continue
		}
		for thick := range g.config.AnchoLineas {
			if y+thick < img.Bounds().Max.Y {
				img.Set(x+i, y+thick, col)
			}
		}
	}
}

func (g *GeneradorTalonarios) dibujarRectangulo(img *image.RGBA, x, y, ancho, alto int, col color.RGBA) {
	for i := range ancho {
		if x+i >= img.Bounds().Max.X {
			continue
		}
		for thick := range g.config.AnchoLineas {
			if y+alto-1-thick < img.Bounds().Max.Y {
				img.Set(x+i, y+alto-1-thick, col)
			}
		}
	}

	for i := range alto {
		if y+i >= img.Bounds().Max.Y {
			continue
		}
		for thick := range g.config.AnchoLineas {
			if x+thick < img.Bounds().Max.X {
				img.Set(x+thick, y+i, col)
			}
		}
		for thick := range g.config.AnchoLineas {
			if x+ancho-1-thick < img.Bounds().Max.X {
				img.Set(x+ancho-1-thick, y+i, col)
			}
		}
	}
}

func (g *GeneradorTalonarios) dibujarTexto(img *image.RGBA, texto string, x, y int, col color.RGBA) {

	metrics := g.config.Fuente.Metrics()
	alturaTexto := metrics.Height.Round()
	yCentrado := y + alturaTexto/4

	point := fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(yCentrado * 64),
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{col},
		Face: g.config.Fuente,
		Dot:  point,
	}

	d.DrawString(texto)
}

func (g *GeneradorTalonarios) guardarImagen(img *image.RGBA, nombreArchivo string) error {
	file, err := os.Create(nombreArchivo)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

func (g *GeneradorTalonarios) GenerarTodos() error {
	fmt.Printf("Generando %d talonarios con %d boletas cada uno...\n",
		g.config.CantidadPaginas, g.config.BoletasPorPagina)

	for i := 1; i <= g.config.CantidadPaginas; i++ {
		fmt.Printf("Generando talonario %d/%d...\n", i, g.config.CantidadPaginas)

		talonario := g.crearTalonario(i)

		img := g.crearImagenTalonario(talonario)

		nombreArchivo := filepath.Join(g.config.CarpetaSalida, fmt.Sprintf("talonario_%03d.png", i))
		if err := g.guardarImagen(img, nombreArchivo); err != nil {
			return fmt.Errorf("error guardando talonario %d: %v", i, err)
		}

		fmt.Printf("  Números: ")
		for j, boleta := range talonario.Boletas {
			if j > 0 {
				fmt.Print(", ")
			}
			fmt.Print(boleta.Formateado)
		}
		fmt.Println()
	}

	fmt.Printf("\n✅ Todos los talonarios generados en: %s\n", g.config.CarpetaSalida)
	return nil
}

func main() {
	config := Config{
		ImagenBase:         "Base.png",
		NumeroMinimo:       0,
		NumeroMaximo:       9999,
		BoletasPorPagina:   10,
		CantidadPaginas:    500,
		CarpetaSalida:      "talonarios",
		AnchoTalonario:     1080,
		AltoTalonario:      1920,
		MargenSuperior:     610,
		MargenInferior:     440,
		MargenIzquierdo:    50,
		MargenDerecho:      50,
		BoletasPorFila:     1,
		AnchoLineas:        5,
		ColorTexto:         color.RGBA{248, 220, 191, 255},
		ColorBorde:         color.RGBA{248, 220, 191, 255},
		RutaFuente:         "calibri-bold.ttf",
		TamanoFuente:       38.0,
		OrientacionBoletas: 0, // 0: izquierda, 1: centro, 2: derecha
	}

	fmt.Println("🎫 Generador de Talonarios de Rifas")
	fmt.Println("===================================")
	fmt.Printf("Rango de números: %04d - %04d\n", config.NumeroMinimo, config.NumeroMaximo)
	fmt.Printf("Boletas por talonario: %d\n", config.BoletasPorPagina)
	fmt.Printf("Cantidad de talonarios: %d\n", config.CantidadPaginas)
	fmt.Printf("Total de números a usar: %d\n\n", config.BoletasPorPagina*config.CantidadPaginas)

	generador, err := NewGeneradorTalonarios(config)
	if err != nil {
		log.Fatal("Error configurando generador:", err)
	}

	if err := generador.GenerarTodos(); err != nil {
		log.Fatal("Error generando talonarios:", err)
	}
}
