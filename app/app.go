package app

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"

	"github.com/calaos/calaos_dns/config"
	"github.com/calaos/calaos_dns/models"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

var (
	e *echo.Echo
)

type TemplateRenderer struct {
	templates *template.Template
}

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func Init(conffile *string) error {
	if err := config.ReadConfig(*conffile); err != nil {
		return fmt.Errorf("Failed to read config file: %v", err)
	}

	e = echo.New()

	/*		renderer := &TemplateRenderer{
				templates: template.Must(template.ParseGlob(config.Conf.General.DataPath + "templates/*.tmpl")),
			}
			e.Renderer = renderer
			e.Static("/static", config.Conf.General.DataPath+"static")
	*/

	//Middlewares
	e.Use(middleware.Logger())
	//e.Use(middleware.Recover())

	//CORS
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
	}))

	//API
	e.POST("/api/register", RegisterDns)
	e.GET("/api/update/:token", UpdateDns)
	e.DELETE("/api/delete/:token", DeleteDns)
	e.POST("/api/letsencrypt", AddLeRecord)
	e.DELETE("/api/letsencrypt", DeleteLeRecord)

	return nil
}

func Run() error {
	return e.Start(":" + strconv.Itoa(config.Conf.General.Port))
}

type RegisterJson struct {
	Mainzone string `json:"mainzone" form:"mainzone" query:"mainzone"`
	Subzones string `json:"subzones" form:"subzones" query:"subzones"`
	Token    string `json:"token" form:"token" query:"token"`
}

func RegisterDns(c echo.Context) (err error) {
	req := &RegisterJson{}
	if err = c.Bind(req); err != nil {
		return err
	}

	err, t := models.RegisterDns(req.Mainzone, req.Subzones, req.Token, c.RealIP())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%v", err))
	}

	req.Token = t

	return c.JSON(http.StatusCreated, req)
}

func UpdateDns(c echo.Context) (err error) {
	token := c.Param("token")

	err = models.UpdateDns(token, c.RealIP())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%v", err))
	}

	return c.NoContent(http.StatusOK)
}

func DeleteDns(c echo.Context) (err error) {
	token := c.Param("token")

	err = models.DeleteDns(token)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%v", err))
	}

	return c.NoContent(http.StatusOK)
}

type LeJson struct {
	Token    string `json:"token" form:"token" query:"token"`
	LeDomain string `json:"le_domain" form:"le_domain" query:"le_domain"`
	LeToken  string `json:"le_token" form:"le_token" query:"le_token"`
}

func AddLeRecord(c echo.Context) (err error) {
	req := &LeJson{}
	if err = c.Bind(req); err != nil {
		return err
	}

	err = models.AddLeRecord(req.Token, req.LeDomain, req.LeToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%v", err))
	}

	return c.NoContent(http.StatusCreated)
}

func DeleteLeRecord(c echo.Context) (err error) {
	req := &LeJson{}
	if err = c.Bind(req); err != nil {
		return err
	}

	err = models.DeleteLeRecord(req.Token, req.LeDomain)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%v", err))
	}

	return c.NoContent(http.StatusOK)
}
