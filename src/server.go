package main 

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

var (
	form url.Values
	header http.Header
	cookies []*http.Cookie
	writer http.ResponseWriter
	request http.Request
	exit_error bool
)

func initServer() {
	if config.Port == 0 {
		config.Port = 80
	}
	listener,err := net.Listen("tcp", config.Domain+":"+strconv.Itoa(config.Port))
	if(err != nil) {
		fmt.Printf("Failed listening on "+config.Domain+":%d, see log for details",config.Port)
		error_log.Fatal(err.Error())
	}
	http.Handle("/", makeHandler(mainHandle))
	http.Handle("/manage",makeHandler(callManageFunction))
	http.Handle("/post",makeHandler(makePost))
	http.Handle("/util",makeHandler(utilHandler))
	http.Serve(listener, nil)
}

func mainHandle(w http.ResponseWriter, r *http.Request) {
	request = *r
	writer = w
	cookies = request.Cookies()
	request.ParseForm()
	form = request.Form
	request_url := request.URL.Path

	filepath := path.Join(config.DocumentRoot, request_url)
	results,err := os.Stat(filepath)

	if err == nil {
		//the file exists, or there is a folder here
		if results.IsDir() {
			found_index := false
			newpath := ""

			//check to see if one of the specified index pages exists
			for i := 0; i < len(config.FirstPage); i++ {
				newpath = path.Join(filepath,config.FirstPage[i])
				_,err := os.Stat(newpath)
				if err == nil {
					//writer.Header().Add("Cache-Control", "max-age=5, must-revalidate")
					serveFile(w, newpath)
					found_index = true
					break
				}
			}

			if !found_index {
				error404()
			}
		} else {
			//the file exists, and is not a folder
			/*extension := getFileExtension(request_url)
			if extension  == "html" || extension == "htm" {
				//writer.Header().Add("Cache-Control", "max-age=5, must-revalidate")
			}*/
			serveFile(w, filepath)
		}
	} else {
		//there is nothing at the requested address
		error404()
	}
}

func utilHandler(writer http.ResponseWriter, request *http.Request) {
	action := request.FormValue("action")
	board := request.FormValue("board")

	if action == "" && request.PostFormValue("delete_btn") != "Delete" && request.PostFormValue("report_btn") != "Report" {
		http.Redirect(writer,request,path.Join(config.SiteWebfolder,"/"),http.StatusFound)
		return
	}
	var posts_arr []string
	for key,_ := range request.PostForm {
		if strings.Index(key,"check") == 0 {
			posts_arr = append(posts_arr,key[5:])
		}
	}
	if request.PostFormValue("delete_btn") == "Delete" {
		file_only := request.FormValue("fileonly") == "on"
		password := md5_sum(request.FormValue("password"))
		rank := getStaffRank()

		if request.FormValue("password") == ""  && rank == 0 {
			exitWithErrorPage(writer, "Password required for post deletion")
			return
		}

		for _,post := range posts_arr {
			var parent_id int
			var filename string
			var filetype string
			var password_checksum string
			var board_id int
			post_int,err := strconv.Atoi(post)

			err = db.QueryRow("SELECT `parentid`,`filename`,`password` FROM `"+config.DBprefix+"posts` WHERE `id` = "+post).Scan(&parent_id,&filename,&password_checksum)
			if err == sql.ErrNoRows {
				//the post has already been deleted
				fmt.Fprintf(writer, "%s has already been deleted\n",post)
				continue
			}
			if err != nil {
				exitWithErrorPage(writer,err.Error())
				return
			}

			err = db.QueryRow("SELECT `id` FROM `"+config.DBprefix+"boards` WHERE `dir` = '"+board+"'").Scan(&board_id)
			if err != nil {
				exitWithErrorPage(writer,err.Error())
				return
			}

			if password != password_checksum && rank == 0 {
				fmt.Fprintf(writer, "Incorrect password for %s\n", post)
				continue
			}
			
			if file_only {
				if filename != "" {
					filetype = filename[strings.Index(filename,".")+1:]
					filename = filename[:strings.Index(filename,".")]
					err := os.Remove(path.Join(config.DocumentRoot,board,"/src/"+filename+"."+filetype))
					if err != nil {
						exitWithErrorPage(writer,err.Error())
						return
					}
					err = os.Remove(path.Join(config.DocumentRoot,board,"/thumb/"+filename+"t."+filetype))
					if err != nil {
						exitWithErrorPage(writer,err.Error())
						return
					}
					_,err = db.Exec("UPDATE `"+config.DBprefix+"posts` SET `filename` = 'deleted' WHERE `id` = "+post)
					if err != nil {
						exitWithErrorPage(writer,err.Error())
						return
					}
				}
				fmt.Fprintf(writer, "Attached image from %s deleted successfully\n", post)
				writer.Header().Add("refresh", "5;url="+request.Referer())
			} else {
				if parent_id > 0 {
					os.Remove(path.Join(config.DocumentRoot,board,"/res/index.html"))
				}
				_,err = db.Exec("DELETE FROM `"+config.DBprefix+"posts` WHERE `id` = "+post)
				if parent_id == 0 {
					err = buildThread(post_int, board_id)
				} else {
					err = buildThread(parent_id,board_id)
				}

				if err != nil {
					exitWithErrorPage(writer,err.Error())
					return
				}
				fmt.Fprintf(writer, "%s deleted successfully\n", post)
				writer.Header().Add("refresh", "5;url="+request.Referer())
			}
		}
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//defer serverError()
		if !exit_error {
			fn(w, r)
			exit_error = false
		} else {
			exit_error = false
		}
	}
}

func exitWithErrorPage(w http.ResponseWriter, err string) {
	error_page_bytes,_ := ioutil.ReadFile("templates/error.html")
	error_page := string(error_page_bytes)
	error_page = strings.Replace(error_page,"{ERRORTEXT}", err,-1)
	fmt.Fprintf(w,error_page)
	exit_error = true
}

func redirect(location string) {
	http.Redirect(writer,&request,location,http.StatusFound)
}

func error404() {
	http.ServeFile(writer, &request, path.Join(config.DocumentRoot, "/error/404.html"))
	error_log.Print("Error: 404 Not Found from " + request.RemoteAddr + " @ " + request.RequestURI)
}

func validReferrer(request http.Request) (valid bool) {
	if request.Referer() == "" || request.Referer()[7:len(config.SiteDomain)+7] != config.SiteDomain {
		valid = false
	} else {
		valid = true
	}
	return
}

func serverError() {
	if _, ok := recover().(error); ok {
		//something went wrong, now we need to throw a 500
		http.ServeFile(writer,&request, path.Join(config.DocumentRoot, "/error/500.html"))
		error_log.Print("Error: 500 Internal Server error from " + request.RemoteAddr + " @ " + request.RequestURI)	
		return
	}
}

func serveFile(w http.ResponseWriter, filepath string) {
	http.ServeFile(w, &request, filepath)
	access_log.Print("Success: 200 from " + request.RemoteAddr + " @ " + request.RequestURI)
}
